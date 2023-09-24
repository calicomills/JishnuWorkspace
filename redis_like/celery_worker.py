from celery import Celery
from celery.schedules import crontab

from db import inmemdb
import logging

logger = logging.getLogger(__name__)
# Create a Celery instance
celery = Celery(
    "myapp",
    broker='redis://localhost:6379/0',  
    backend='redis://localhost:6379/0',  
)

@celery.task
def hello():
     print("hello")
     #inmemdb.create_attrs(x=2)

@celery.task
def delete_entry():
        # Get and print class attributes
        print(inmemdb.data)
        for entry in inmemdb.data:
                 print(entry)
                 try:
                    inmemdb.remove(entry)
                 except Exception as e:
                      print(e)

# Define a periodic task to run delete_entry evecry 5 minutes
celery.conf.beat_schedule = {
    'cleanup-every-2-minutes': {
        'task': 'celery_worker.delete_entry',  # Task to run
        'schedule': 10,  # Run every 5 minutes  
    },
}

if __name__ == '__main__':
    celery.start()
