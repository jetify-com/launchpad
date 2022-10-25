import os
import socket
from typing import Any, Dict, List, Optional

from fastapi import FastAPI, Header
import uvicorn

from jetpack import jetroutine

app = FastAPI()


@app.get("/")
async def root(x_forwarded_for: Optional[List[str]] = Header(None)) -> Dict[str, str]:
    return {
        "message": "Hello World!",
        "secret": os.getenv("SECRET_MIKE"),
        "ip": socket.gethostbyname(socket.gethostname()),
        "x_forwarded_for": x_forwarded_for,
    }


@app.get("/create-job")
async def create_job() -> Any:
    return await my_job("Jetpack team")


@jetroutine
async def my_job(caller: str) -> Dict[str, str]:
    return {"message": f"Hello World from {caller}!"}


if __name__ == "__main__":
    uvicorn.run("jetpack_main:app", host="0.0.0.0", port=8080, reload=True)
