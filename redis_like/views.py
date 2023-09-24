"""
TODO:
    1. login and authentication to the API
    2. rate limiter
    3. making it into a CLI too
    4. add celery for DB transactions
"""

from flask import Flask, request, Response, render_template, redirect, url_for, jsonify, flash
from flask_login import LoginManager, login_user, login_required, logout_user, current_user
import json
import os
from rate_limiter import rate_limiter, cleaner_thread

from user import User, users, user_data
from db import inmemdb
from constants import DUMP_FILE_PATH


app = Flask(__name__)
app.jinja_env.auto_reload = True
app.config['TEMPLATES_AUTO_RELOAD'] = True
app.config['SECRET_KEY'] = 'sldkf'

# Initialize Flask-Login
login_manager = LoginManager()
login_manager.login_view = "login"
login_manager.init_app(app)

# Views

@login_manager.user_loader
def load_user(user_id):
    print(user_id)
    return users.get(user_id)


@app.route("/login", methods=["POST", "GET"])
def login():
    # Implement your login logic here
    # Check if the user is authenticated, and if so, call login_user(user) to log them in.
    # Replace this logic with your actual authentication process.
    if request.method == "POST":
        data = request.get_json()
        user_name = data.get("username")
        password = hash(data.get("password"))
        is_authenticated = False
        if user_data.get(user_name, None):
            if password == user_data[user_name]:
                is_authenticated = True

        if is_authenticated:
            user = User(1, user_name, password)  # Replace 'user_id' with the actual user ID
            login_user(user)
            try:
                return Response(status=200)
            except Exception as e:
                print(e)
            return "Redirecting"
    else:
        return "Credentials not correct"
    
@app.route("/")
def index():
    return render_template("login.html")


@app.route("/logout")
def logout():
    logout_user()
    return render_template("logout.html")


@app.route("/input", methods=["GET", "POST"])
def home():
    #return render_template("template.html")
    try:
        return render_template('input.html')
    except Exception as e:
        # Handle the error, log it, or return an error message
        print(e)
        return f"Error: {str(e)}", 500 
    


@app.route("/send", methods=["POST"])
def update_db():
    print([key for key in dir(inmemdb) if not key.startswith("__")])
    data = request.get_json()
    if not rate_limiter(request, 2):
        error_msg = "Try after some time"
        return Response(error_msg, status=400, content_type='text/plain')
    else:
        key = data.get("key", None)
        val = data.get("value", None)
        persist = data.get("persist", None)
        inmemdb.create_attrs(**{f"{key}": val})
        #print([key for key in dir(inmemdb) if not key.startswith("__")])
        if persist:
            try:
                with open(DUMP_FILE_PATH, "r+") as json_file:
                    data = json.load(json_file)
                    if not data:
                        data = {}
                    data[key] = val
                    json_file.seek(0)
                    json.dump(json_file, data, indent=4)
                    json_file.truncate()
            except Exception as e:
                with open(DUMP_FILE_PATH, "w") as json_file:
                    data = {}
                    data[key] = val
                    json.dump(data, json_file, indent=4)
                return Response(status=200)
        return Response(status=200)


@app.route("/<key>", methods=["GET"])
def get_data_from_db(key):
    x = inmemdb.get_attr(key) 
    return x if x else "Not found"

if __name__ == "__main__":
    from threading import Thread
    Thread(target=cleaner_thread, args=(2,)).start()
    app.run(debug=True, threaded=True)
    
    
