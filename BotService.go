package Bot

import (
	Database "Bot/Database"
	Logger "Bot/Logger"
	Schedule "Bot/Schedule"
	"strings"
	"sync"
	"time"

	tgBotAPI "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// AntiFloodCooldown is the minimum gap between two accepted messages from
	// one user. Anything faster is dropped, so a user holding down a button
	// cannot make the bot spam Telegram on their behalf.
	AntiFloodCooldown = 700 * time.Millisecond

	// idleTTL is how long a chat may sit untouched before its per-chat
	// bookkeeping is released, and janitorInterval how often that is checked.
	// Without this the per-chat maps would only ever grow.
	idleTTL         = 2 * time.Hour
	janitorInterval = 30 * time.Minute

	// maxTransientPerChat caps the pending-deletion list. A week view produces
	// eight messages, so this leaves plenty of room for retries without letting
	// an undeletable message be chased indefinitely.
	maxTransientPerChat = 50
)

type BotService struct {
	BotAPI   *tgBotAPI.BotAPI
	Database Database.Database
	Schedule *Schedule.Cache
	PhysEd   *Schedule.PhysEdCache
	Location *time.Location

	// Calendar decides which weeks are "в" and which are "н".
	Calendar Schedule.Calendar

	// PhysEdVenue is where the special-medical PE classes are held; it is the
	// same for every slot, so it lives in the config rather than the sheet.
	PhysEdVenue string

	// ScheduleURL points at the source spreadsheet, offered as a button on the
	// pinned message so students can check the original.
	ScheduleURL string

	// ShowExams toggles the "Экзамены" button in the main menu, so it can be
	// switched on for the session period and off again afterwards.
	ShowExams bool

	// chatsMutex guards chats; entries are touched from one goroutine per
	// update, so unsynchronised access would be a data race.
	chatsMutex sync.Mutex
	chats      map[int64]*chatState
}

// chatState is everything the bot remembers about one chat in memory. Keeping
// it in a single map (rather than one map per field) means the janitor releases
// a chat atomically and the pieces can never disagree about who exists.
type chatState struct {
	// lock serialises handling for this chat, so replies arrive in order.
	lock sync.Mutex

	// lastAction is any update at all and drives idle cleanup, while
	// lastMessage counts only text messages and drives anti-flood. They are
	// separate so that tapping an inline button cannot throttle the message
	// the user types straight afterwards.
	lastAction  time.Time
	lastMessage time.Time

	// transient holds the ids of every message that may be wiped on this chat's
	// next action - the bot's own replies plus the user's button presses.
	// The anchor message is deliberately not in here.
	transient []int
}

// Options carries the settings a BotService needs beyond its dependencies,
// so adding one does not mean touching every call site.
type Options struct {
	Location    *time.Location
	Calendar    Schedule.Calendar
	ShowExams   bool
	PhysEdVenue string
	ScheduleURL string
}

func NewBotService(botAPI *tgBotAPI.BotAPI, database Database.Database, sched *Schedule.Cache, physEd *Schedule.PhysEdCache, opts Options) *BotService {
	return &BotService{
		BotAPI:      botAPI,
		Database:    database,
		Schedule:    sched,
		PhysEd:      physEd,
		Location:    opts.Location,
		Calendar:    opts.Calendar,
		PhysEdVenue: opts.PhysEdVenue,
		ScheduleURL: opts.ScheduleURL,
		ShowExams:   opts.ShowExams,
		chats:       map[int64]*chatState{},
	}
}

func (bs *BotService) Start() {
	u := tgBotAPI.NewUpdate(0)
	u.Timeout = 20
	// Ask for both update types explicitly. Left unset, the library serialises
	// allowed_updates as null, and the inline arrows depend on callback_query
	// actually being delivered - so this is not a detail worth leaving to chance.
	u.AllowedUpdates = []string{"message", "callback_query"}

	bs.startJanitor()

	updates := bs.BotAPI.GetUpdatesChan(u)
	Logger.Info("bot started, listening for updates")

	for update := range updates {
		go bs.handleUpdate(update)
	}
}

func (bs *BotService) Final() {
	bs.BotAPI.StopReceivingUpdates()
	Logger.Info("bot stopped")
}

// handleUpdate processes one Telegram update in its own goroutine. A panic here
// must never escape, or it would take the whole bot down with it.
func (bs *BotService) handleUpdate(update tgBotAPI.Update) {
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("recovered from panic while handling update: %v", r)
		}
	}()

	Logger.Debug("update %d received (callback=%v, message=%v)",
		update.UpdateID, update.CallbackQuery != nil, update.Message != nil)

	switch {
	case update.CallbackQuery != nil:
		bs.handleCallback(update.CallbackQuery)

	case update.Message != nil && update.Message.Chat != nil && update.Message.PinnedMessage != nil:
		// Pinning the anchor makes Telegram post a "pinned a message" notice.
		// Remove it so the chat stays as clean as the rest of this does.
		bs.deleteMessage(update.Message.Chat.ID, update.Message.MessageID)

	case update.Message != nil && update.Message.Chat != nil && update.Message.Text != "":
		bs.handleMessage(update.Message)
	}
}

func (bs *BotService) handleMessage(msg *tgBotAPI.Message) {
	chatID := msg.Chat.ID
	// Commands are never throttled: /start is the way out of a stuck chat and
	// must always work, even right after another action.
	if tooSoon := bs.touchMessage(chatID); tooSoon && !strings.HasPrefix(msg.Text, "/") {
		Logger.Warn("chat %d: message %q dropped by anti-flood", chatID, msg.Text)
		return
	}

	// Serialise everything for one chat so replies always arrive in order.
	state := bs.chat(chatID)
	state.lock.Lock()
	defer state.lock.Unlock()

	Logger.Info("chat %d: %q", chatID, msg.Text)

	// Wipe what the previous action left behind, and the user's own message
	// with it, so the chat only ever holds the anchor plus the newest reply.
	bs.clearTransient(chatID)
	bs.deleteMessage(chatID, msg.MessageID)

	bs.handleCommand(chatID, msg.Text)
}

func (bs *BotService) handleCallback(cb *tgBotAPI.CallbackQuery) {
	if cb.Message == nil || cb.Message.Chat == nil {
		// Telegram omits the message for very old ones; nothing to edit then.
		Logger.Warn("callback %q arrived without a message, ignoring", cb.Data)
		return
	}
	chatID := cb.Message.Chat.ID

	// Always answer, otherwise the button spins on the client.
	if _, err := bs.BotAPI.Request(tgBotAPI.NewCallback(cb.ID, "")); err != nil {
		Logger.Warn("chat %d: failed to answer callback: %v", chatID, err)
	}

	// Deliberately not throttled: paging through days is meant to feel instant,
	// and each press only rewrites one existing message. The chat is still
	// touched so the janitor sees it as active.
	bs.touchAction(chatID)

	state := bs.chat(chatID)
	state.lock.Lock()
	defer state.lock.Unlock()

	Logger.Info("chat %d: callback %q", chatID, cb.Data)
	bs.handleCallbackData(chatID, cb.Message.MessageID, cb.Data)
}

// chat returns the chat's state, creating it on first contact.
func (bs *BotService) chat(chatID int64) *chatState {
	bs.chatsMutex.Lock()
	defer bs.chatsMutex.Unlock()

	state, ok := bs.chats[chatID]
	if !ok {
		state = &chatState{}
		bs.chats[chatID] = state
	}
	return state
}

// touchMessage records an incoming text message and reports whether it followed
// the previous one too closely to be worth handling.
func (bs *BotService) touchMessage(chatID int64) (tooSoon bool) {
	state := bs.chat(chatID)

	bs.chatsMutex.Lock()
	defer bs.chatsMutex.Unlock()

	now := time.Now()
	previous := state.lastMessage
	state.lastMessage, state.lastAction = now, now
	return !previous.IsZero() && now.Sub(previous) < AntiFloodCooldown
}

// touchAction records any other kind of update. It feeds the idle clock only,
// never the anti-flood one, so inline buttons stay instant.
func (bs *BotService) touchAction(chatID int64) {
	state := bs.chat(chatID)

	bs.chatsMutex.Lock()
	state.lastAction = time.Now()
	bs.chatsMutex.Unlock()
}

// startJanitor releases bookkeeping for chats that have gone quiet, so the
// in-memory state cannot grow without bound as users come and go.
func (bs *BotService) startJanitor() {
	go func() {
		for {
			time.Sleep(janitorInterval)
			bs.pruneIdleChats()
		}
	}()
}

func (bs *BotService) pruneIdleChats() {
	bs.chatsMutex.Lock()
	defer bs.chatsMutex.Unlock()

	cutoff := time.Now().Add(-idleTTL)
	pruned := 0
	for chatID, state := range bs.chats {
		if state.lastAction.After(cutoff) {
			continue
		}
		// Every handler touches the chat before taking its lock, so an in-flight
		// update always looks recent here. TryLock is the belt to that braces:
		// a held lock means the chat is busy and must not be dropped.
		if !state.lock.TryLock() {
			continue
		}
		state.lock.Unlock()
		delete(bs.chats, chatID)
		pruned++
	}
	if pruned > 0 {
		Logger.Debug("janitor: released state for %d idle chat(s)", pruned)
	}
}

func (bs *BotService) trackTransient(chatID int64, messageIDs ...int) {
	state := bs.chat(chatID)

	bs.chatsMutex.Lock()
	state.transient = append(state.transient, messageIDs...)
	// Retries are bounded: if something truly refuses to be deleted, the oldest
	// ids are dropped rather than kept and re-attempted on every single action.
	if extra := len(state.transient) - maxTransientPerChat; extra > 0 {
		state.transient = append([]int(nil), state.transient[extra:]...)
	}
	bs.chatsMutex.Unlock()
}

// takeTransient hands over the chat's pending message ids and clears the list.
func (bs *BotService) takeTransient(chatID int64) []int {
	state := bs.chat(chatID)

	bs.chatsMutex.Lock()
	defer bs.chatsMutex.Unlock()

	ids := state.transient
	state.transient = nil
	return ids
}

// clearTransient deletes every tracked message for the chat. The anchor is
// never tracked, so it always survives. This is what stops the chat filling up:
// each new action wipes what the previous one left behind, including the seven
// messages a week view produces.
//
// A delete can fail for a passing reason - most often Telegram's rate limit,
// which the seven messages of a week view are quite capable of hitting. Those
// ids are put back so the next action retries them; otherwise a single throttled
// call would strand that message in the chat forever.
func (bs *BotService) clearTransient(chatID int64) {
	var retry []int
	for _, id := range bs.takeTransient(chatID) {
		if !bs.deleteMessage(chatID, id) {
			retry = append(retry, id)
		}
	}
	if len(retry) > 0 {
		Logger.Debug("chat %d: %d message(s) will be retried on the next action", chatID, len(retry))
		bs.trackTransient(chatID, retry...)
	}
}

// permanentDeleteFailures are the answers that mean retrying is pointless:
// the message is already gone, or Telegram will never let us remove it.
var permanentDeleteFailures = []string{
	"message to delete not found",
	"message can't be deleted",
	"MESSAGE_ID_INVALID",
	"chat not found",
	"bot was blocked by the user",
	"user is deactivated",
}

// deleteMessage removes one message. It reports false only when the message may
// still exist and is worth another attempt later.
func (bs *BotService) deleteMessage(chatID int64, messageID int) bool {
	if messageID == 0 {
		return true
	}

	_, err := bs.BotAPI.Request(tgBotAPI.NewDeleteMessage(chatID, messageID))
	if err == nil {
		return true
	}

	if apiErr, ok := err.(*tgBotAPI.Error); ok {
		for _, permanent := range permanentDeleteFailures {
			if strings.Contains(apiErr.Message, permanent) {
				Logger.Debug("chat %d: message %d cannot be deleted: %v", chatID, messageID, err)
				return true // nothing to retry
			}
		}
	}

	Logger.Debug("chat %d: delete of message %d failed, will retry: %v", chatID, messageID, err)
	return false
}

// pinMessage pins the anchor so it stays at the top of the chat. Pinning is a
// convenience, not a requirement, so a refusal is only worth a debug line.
func (bs *BotService) pinMessage(chatID int64, messageID int) {
	pin := tgBotAPI.PinChatMessageConfig{
		ChatID:              chatID,
		MessageID:           messageID,
		DisableNotification: true,
	}
	if _, err := bs.BotAPI.Request(pin); err != nil {
		Logger.Debug("chat %d: could not pin message %d: %v", chatID, messageID, err)
	}
}

func (bs *BotService) unpinMessage(chatID int64, messageID int) {
	unpin := tgBotAPI.UnpinChatMessageConfig{ChatID: chatID, MessageID: messageID}
	if _, err := bs.BotAPI.Request(unpin); err != nil {
		Logger.Debug("chat %d: could not unpin message %d: %v", chatID, messageID, err)
	}
}

// send delivers an HTML message and tracks it for the next cleanup.
// Pass nil as keyboard to leave the current one in place.
func (bs *BotService) send(chatID int64, text string, keyboard interface{}) int {
	id := bs.sendRaw(chatID, text, keyboard)
	if id != 0 {
		bs.trackTransient(chatID, id)
	}
	return id
}

// sendRaw delivers a message without tracking it, for the anchor message.
func (bs *BotService) sendRaw(chatID int64, text string, keyboard interface{}) int {
	msg := tgBotAPI.NewMessage(chatID, text)
	msg.ParseMode = tgBotAPI.ModeHTML
	msg.DisableWebPagePreview = true
	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}

	sent, err := bs.BotAPI.Send(msg)
	if err != nil {
		Logger.Error("chat %d: send failed: %v", chatID, err)
		return 0
	}
	return sent.MessageID
}

// editMessage rewrites a message in place, which is how day navigation avoids
// adding anything to the chat. It reports false when the edit could not be made.
func (bs *BotService) editMessage(chatID int64, messageID int, text string, keyboard *tgBotAPI.InlineKeyboardMarkup) bool {
	edit := tgBotAPI.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = tgBotAPI.ModeHTML
	edit.DisableWebPagePreview = true
	if keyboard != nil {
		edit.ReplyMarkup = keyboard
	}

	if _, err := bs.BotAPI.Request(edit); err != nil {
		// Telegram rejects an edit that would not change anything; that is a
		// no-op for us, not a failure.
		if apiErr, ok := err.(*tgBotAPI.Error); ok && strings.Contains(apiErr.Message, "message is not modified") {
			return true
		}
		Logger.Warn("chat %d: edit of message %d failed: %v", chatID, messageID, err)
		return false
	}
	return true
}
