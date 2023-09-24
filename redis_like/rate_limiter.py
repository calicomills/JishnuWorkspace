"""
a rate limiter should restrict the number of api calls from a client.
"""
from db import inmemdb 

incr = lambda x: x+1

def rate_limiter(request, num_reqs=5):
    print("inmemdb",inmemdb.ip_data)
    ip = request.remote_addr
    print("IP:", ip,hash(ip), inmemdb.data)
    counter = inmemdb.get_ip_attr(hash(ip))
    print(counter)
    if counter == num_reqs:
        return False
    if counter:
        print("incrementing by one")
        inmemdb.create_update_ip(**{f"{hash(ip)}":incr(counter)})
    else:
        inmemdb.create_update_ip(**{f"{hash(ip)}":1})
    return True
    
def cleaner_thread(limit):
    import time
    print("cleaner thread started!")
    while True:
        keys = list(inmemdb.ip_data.keys())
        print("keys in loop",keys)
        for key in keys:
            if inmemdb.ip_data[key] >= limit:
                inmemdb.ip_data.pop(key)
        time.sleep(30)
            


            
            


