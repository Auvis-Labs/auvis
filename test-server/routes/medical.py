from fastapi import APIRouter, Depends
from auth import get_current_user
from data import MEDICAL_DATA

router = APIRouter(prefix="/medical", tags=["medical"])


@router.get("/records")
def list_records(current_user: dict = Depends(get_current_user)):
    """
    Returns medical records. Contains: diagnosis, medications,
    SSN, patient email, doctor email, insurance ID.
    HIPAA-class PII — good for full redact / deny tests.
    """
    return {"data": MEDICAL_DATA, "meta": {"total": len(MEDICAL_DATA)}}


@router.get("/records/{record_id}")
def get_record(
    record_id: str,
    current_user: dict = Depends(get_current_user)
):
    """Single medical record by ID."""
    record = next((r for r in MEDICAL_DATA if r["id"] == record_id), None)
    if not record:
        from fastapi import HTTPException
        raise HTTPException(status_code=404, detail="Record not found")
    return record