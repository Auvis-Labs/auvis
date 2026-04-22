from fastapi import APIRouter, Depends, Query
from auth import get_current_user
from data import USERS_DATA

router = APIRouter(prefix="/users", tags=["users"])


@router.get("/")
def list_users(
    region: str = Query(None, description="Filter by region: NG, US"),
    current_user: dict = Depends(get_current_user)
):
    """
    Returns all users. Contains: name, email, phone, SSN,
    date_of_birth, address. Rich PII — good for masking tests.
    """
    results = USERS_DATA
    if region:
        results = [u for u in results if u["region"] == region]
    return {"data": results, "meta": {"total": len(results)}}


@router.get("/{user_id}")
def get_user(
    user_id: str,
    current_user: dict = Depends(get_current_user)
):
    """Single user by ID."""
    user = next((u for u in USERS_DATA if u["id"] == user_id), None)
    if not user:
        from fastapi import HTTPException
        raise HTTPException(status_code=404, detail="User not found")
    return user