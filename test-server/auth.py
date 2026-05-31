import json
from datetime import datetime, timedelta
from typing import Optional
from jose import JWTError, jwt
from fastapi import HTTPException, Security
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from data import USERS_DATA

SECRET_KEY = "auvis-test-secret-do-not-use-in-production"
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_MINUTES = 60

security = HTTPBearer()

# Simulated user store — in a real system this is a database
USERS = {
    "admin": {
        "password": "admin123",
        "role": "admin",
        "user_id": "usr_001"
    },
    "analyst": {
        "password": "analyst123",
        "role": "analyst",
        "user_id": "usr_002"
    },
    "doctor": {
        "password": "doctor123",
        "role": "doctor",
        "user_id": "usr_003"
    },
    "auditor": {
        "password": "auditor123",
        "role": "auditor",
        "user_id": "usr_004"
    },
}


def create_access_token(user_id: str, role: str) -> str:
    payload = {
        "sub": user_id,
        "role": role,
        "iat": datetime.utcnow(),
        "exp": datetime.utcnow() + timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES),
    }
    
    # Enrich payload with all attributes of the user from data.py
    user_info = next((u for u in USERS_DATA if u["id"] == user_id), None)
    if user_info:
        for k, v in user_info.items():
            if k not in ["id", "role"]:
                if isinstance(v, dict):
                    payload[k] = json.dumps(v)
                else:
                    payload[k] = str(v)
                    
    return jwt.encode(payload, SECRET_KEY, algorithm=ALGORITHM)


def decode_token(token: str) -> dict:
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
        return payload
    except JWTError:
        raise HTTPException(status_code=401, detail="Invalid or expired token")


def get_current_user(
    credentials: HTTPAuthorizationCredentials = Security(security)
) -> dict:
    return decode_token(credentials.credentials)