from flask_login import UserMixin

class User(UserMixin):
    def __init__(self, user_id = 1, name = "", password= ""):
        self.name = name
        self.password = password
        self.id = user_id

users = {1: User("jishnu", hash("123"))}

user_data = {"jishnu": hash("123")}


