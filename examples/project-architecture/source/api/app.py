"""Illustrative service source: store a job, then enqueue its ID."""
import json
import os
import uuid

import psycopg
import redis
from fastapi import FastAPI

app = FastAPI()
queue = redis.Redis.from_url(os.environ["REDIS_URL"])


@app.post("/jobs")
def create_job(payload: dict):
    job_id = str(uuid.uuid4())
    with psycopg.connect(
        host=os.environ["PGHOST"], port=int(os.environ.get("PGPORT", "5432")),
        dbname="app", user="app", password=os.environ["PGPASSWORD"],
    ) as connection:
        connection.execute("CREATE TABLE IF NOT EXISTS jobs (id TEXT PRIMARY KEY, payload JSONB)")
        connection.execute("INSERT INTO jobs VALUES (%s, %s)", (job_id, json.dumps(payload)))
    queue.rpush("jobs", job_id)
    return {"id": job_id}
