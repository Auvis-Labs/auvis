from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from auth import USERS, create_access_token
from routes.users import router as users_router
from routes.transactions import router as transactions_router
from routes.medical import router as medical_router

app = FastAPI(
    title="AUVIS Test Server",
    description="Realistic API with PII-rich payloads for testing AUVIS masking policies.",
    version="1.0.0"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# Keep compatibility with launch commands that expect `main:main`.
main = app

app.include_router(users_router)
app.include_router(transactions_router)
app.include_router(medical_router)


class LoginRequest(BaseModel):
    username: str
    password: str


@app.post("/auth/token")
def login(req: LoginRequest):
    """
    Exchange username + password for a JWT.
    
    Test credentials:
      admin    / admin123    → role: admin
      analyst  / analyst123  → role: analyst  
      doctor   / doctor123   → role: doctor
      auditor  / auditor123  → role: auditor
    """
    user = USERS.get(req.username)
    if not user or user["password"] != req.password:
        raise HTTPException(status_code=401, detail="Invalid credentials")

    token = create_access_token(
        user_id=user["user_id"],
        role=req.username
    )
    return {
        "access_token": token,
        "token_type": "bearer",
        "role": req.username,
        "user_id": user["user_id"]
    }


last_llm_request = None


class Message(BaseModel):
    role: str
    content: str


class ChatRequest(BaseModel):
    messages: list[Message]


@app.post("/llm/chat")
def chat(req: ChatRequest):
    global last_llm_request
    last_llm_request = req.dict()
    
    content = ""
    if req.messages:
        content = req.messages[-1].content
        
    return {
        "choices": [
            {
                "message": {
                    "role": "assistant",
                    "content": f"Thank you! I received: {content}"
                }
            }
        ]
    }


@app.get("/llm/last_request")
def get_last_request():
    return {"last_request": last_llm_request}


@app.get("/health")
def health():
    return {"status": "ok", "service": "auvis-test-server"}