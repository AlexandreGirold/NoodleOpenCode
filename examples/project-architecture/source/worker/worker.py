"""Illustrative worker: BLPOP opens a client connection to Redis, not the reverse."""
import os

import httpx
import redis

queue = redis.Redis.from_url(os.environ["REDIS_URL"])
with httpx.Client(timeout=10) as client:
    while True:
        _, job_id = queue.blpop("jobs")
        response = client.post(os.environ["VALIDATION_URL"], json={"id": job_id.decode()})
        response.raise_for_status()
