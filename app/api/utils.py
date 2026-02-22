from datetime import date

DAYS_NAMES = [
    "Monday",
    "Tuesday",
    "Wednesday",
    "Thursday",
    "Thursday",
    "Friday",
    "Saturday",
    "Sunday"
]

def generate_valid_schedule_json(data):
    ret = {
        "week_v": {
            "Monday": [],
            "Tuesday": [],
            "Wednesday": [],
            "Thursday": [],
            "Friday": [],
            "Saturday": [],
            "Sunday": []
        },
        "week_n": {
            "Monday": [],
            "Tuesday": [],
            "Wednesday": [],
            "Thursday": [],
            "Friday": [],
            "Saturday": [],
            "Sunday": []
        }
    }

    for day in data:
        if day["week_type"] == "в" and day["subject"] != "":
            match day["day"]:
                case "Понедельник":
                    ret["week_v"]["Monday"].append(day)
                
                case "Вторник":
                    ret["week_v"]["Tuesday"].append(day)
                
                case "Среда":
                    ret["week_v"]["Wednesday"].append(day)
                
                case "Четверг":
                    ret["week_v"]["Thursday"].append(day)
                
                case "Пятница":
                    ret["week_v"]["Friday"].append(day)
                
                case "Суббота":
                    ret["week_v"]["Saturday"].append(day)
                
                case "Воскресенье":
                    ret["week_v"]["Sunday"].append(day)
        
        if day["week_type"] == "н" and day["subject"] != "":
            match day["day"]:
                case "Понедельник":
                    ret["week_n"]["Monday"].append(day)
                
                case "Вторник":
                    ret["week_n"]["Tuesday"].append(day)
                
                case "Среда":
                    ret["week_n"]["Wednesday"].append(day)
                
                case "Четверг":
                    ret["week_n"]["Thursday"].append(day)
                
                case "Пятница":
                    ret["week_n"]["Friday"].append(day)
                
                case "Суббота":
                    ret["week_n"]["Saturday"].append(day)
                
                case "Воскресенье":
                    ret["week_n"]["Sunday"].append(day)
    
    return ret


def get_current_week_type(start_date: date = date(2025, 9, 1), target_date: date | None = None):
    weeks_passed = (target_date - start_date).days // 7
    return "v" if weeks_passed % 2 == 0 else "n"



    