from fastapi import APIRouter, Depends, Query
from auth import get_current_user
from data import TRANSACTIONS_DATA

router = APIRouter(prefix="/transactions", tags=["transactions"])


@router.get("/")
def list_transactions(
    user_id: str = Query(None, description="Filter by user_id"),
    status: str = Query(None, description="Filter by status"),
    current_user: dict = Depends(get_current_user)
):
    """
    Returns transactions. Contains: card_number, user_email,
    user_name, ip_address. PCI + PII — good for card masking tests.
    """
    results = TRANSACTIONS_DATA
    if user_id:
        results = [t for t in results if t["user_id"] == user_id]
    if status:
        results = [t for t in results if t["status"] == status]
    return {"data": results, "meta": {"total": len(results)}}


@router.get("/{transaction_id}")
def get_transaction(
    transaction_id: str,
    current_user: dict = Depends(get_current_user)
):
    """Single transaction by ID."""
    txn = next((t for t in TRANSACTIONS_DATA if t["id"] == transaction_id), None)
    if not txn:
        from fastapi import HTTPException
        raise HTTPException(status_code=404, detail="Transaction not found")
    return txn