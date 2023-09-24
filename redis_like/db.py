import json

from constants import DUMP_FILE_PATH

class inmemDB:
    _instance = None

    def __new__(cls):
        if cls._instance is None:
            cls._instance = super(inmemDB, cls).__new__(cls)
            cls._instance.data = {}
            cls._instance.ip_data = {}
        return cls._instance

    def create_attrs(self, **attrs):
        for key, val in attrs.items():
            self.data[key] = val
        print(self.data)
    
    def create_update_ip(self, **attrs):
        for key, val in attrs.items():
            self.ip_data[key] = val
        print(self.ip_data)

    def get_ip_attr(self, key):
        ret = self.ip_data.get(str(key), None)
        return ret
    
    def get_attr(self, key):
        ret = None
        try:
            ret = self.data.get(str(key), None)
            print(self.data, key, ret)
        except AttributeError as ae:
            # also check in dump file
            try:
                with open(DUMP_FILE_PATH, "r+") as json_file:
                    data = json.load(json_file)
                    ret = data.get(key, None)
            except json.decoder.JSONDecodeError as json_err:
                ret = None
        return ret
    

    def remove(self, key):
        if self.data.get(str(key), None):
            self.data.pop(str(key))
            print("Deleted", key)

inmemdb = inmemDB()