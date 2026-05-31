import os
import sys
import time
import json
import subprocess
import urllib.request
import urllib.error

# Ensure we can import from the current directory
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from auth import create_access_token

def run_e2e_test():
    print("→ Initializing End-to-End PII Masking Test...")

    # Free up port 8080 and 8000 in case they are already in use by background processes
    print("→ Cleaning up any existing auvis or uvicorn processes to avoid port collisions...")
    try:
        subprocess.run(["pkill", "-f", "auvis"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(["pkill", "-f", "uvicorn"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(0.5) # allow sockets to release
        print("✓ Port cleanup completed.")
    except Exception:
        pass

    # Define paths
    workspace_dir = "/Users/Cridiv/Documents/Auvis"
    audit_log_path = os.path.join(workspace_dir, "audit.jsonl")

    # Clean existing audit log before test run to ensure a clean assertion state
    if os.path.exists(audit_log_path):
        os.remove(audit_log_path)
        print("✓ Cleaned existing audit log.")

    # 1. Start mock upstream FastAPI server on port 8000
    print("→ Starting mock upstream FastAPI server on port 8000...")
    fastapi_proc = subprocess.Popen(
        ["uvicorn", "main:app", "--port", "8000"],
        cwd=os.path.join(workspace_dir, "test-server"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE
    )

    # 2. Wait for FastAPI upstream to be healthy
    print("→ Waiting for mock upstream to become ready...")
    ready = False
    for _ in range(50):
        try:
            with urllib.request.urlopen("http://localhost:8000/health") as r:
                if r.status == 200:
                    ready = True
                    break
        except Exception:
            time.sleep(0.1)

    if not ready:
        fastapi_proc.terminate()
        raise RuntimeError("✗ Failed to start FastAPI mock upstream server")
    print("✓ Mock upstream server is healthy and ready.")

    # 3. Start Auvis Dev Proxy on port 8080
    print("→ Starting Auvis dev gateway on port 8080...")
    env = os.environ.copy()
    env["AUVIS_UPSTREAM"] = "http://localhost:8000"
    env["AUVIS_PORT"] = "8080"
    env["AUVIS_AUDIT_FILE"] = audit_log_path

    # Run the compiled binary we just built
    auvis_proc = subprocess.Popen(
        ["./auvis", "dev", "--config", "auvis.yaml"],
        cwd=os.path.join(workspace_dir, "auvis-infra"),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE
    )

    # 4. Wait for Auvis Dev Proxy to be ready
    print("→ Waiting for Auvis proxy to become ready...")
    ready = False
    for _ in range(50):
        try:
            # Hitting /users without auth should return 401 Unauthorized, proving proxy is listening
            with urllib.request.urlopen("http://localhost:8080/users") as r:
                pass
        except urllib.error.HTTPError as e:
            if e.code == 401:
                ready = True
                break
        except Exception:
            time.sleep(0.1)

    if not ready:
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise RuntimeError("✗ Failed to start Auvis dev proxy server")
    print("✓ Auvis dev proxy is healthy and ready.")

    # 5. Generate signed JWT Analyst Token
    print("→ Generating JWT Analyst token...")
    token = create_access_token(user_id="usr_101", role="analyst")
    print(f"✓ Token generated: {token[:20]}...")

    # 6. Send request through the proxy to /users
    print("→ Sending GET request to http://localhost:8080/users through the proxy...")
    req = urllib.request.Request(
        "http://localhost:8080/users",
        headers={
            "Authorization": f"Bearer {token}",
            "X-Auvis-Upstream": "http://localhost:8000"
        }
    )

    try:
        with urllib.request.urlopen(req) as response:
            status_code = response.status
            response_bytes = response.read()
    except urllib.error.HTTPError as e:
        status_code = e.code
        response_bytes = e.read()
    except Exception as e:
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise RuntimeError(f"✗ Network error connecting to proxy: {e}")

    print(f"✓ Proxy responded with HTTP status code: {status_code}")

    if status_code != 200:
        auvis_proc.terminate()
        fastapi_proc.terminate()
        time.sleep(0.5)
        try:
            import fcntl
            fd = auvis_proc.stderr.fileno()
            fl = fcntl.fcntl(fd, fcntl.F_GETFL)
            fcntl.fcntl(fd, fcntl.F_SETFL, fl | os.O_NONBLOCK)
            err_logs = auvis_proc.stderr.read().decode()
            print("\n→ Auvis Gateway Stderr Logs:")
            print(err_logs)
        except Exception as e:
            print(f"(could not read proxy logs: {e})")
        raise AssertionError(f"✗ Expected HTTP 200 from proxy, got {status_code}. Response: {response_bytes.decode()}")

    # 7. Parse and Assert Response Masking
    response_data = json.loads(response_bytes.decode())
    data_list = response_data.get("data", [])
    if not data_list:
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise AssertionError(f"✗ Expected non-empty data list in response: {response_data}")

    user = data_list[0]
    email = user.get("email")
    ssn = user.get("ssn")
    phone = user.get("phone")
    dob = user.get("date_of_birth")
    street = user.get("address", {}).get("street")

    print("\n→ Verifying Masking Results:")
    print(f"  - Email field: {email}")
    print(f"  - SSN field: {ssn}")
    print(f"  - Phone field: {phone}")
    print(f"  - Date of Birth field: {dob}")
    print(f"  - Address Street field: {street}")

    # Assert email is masked: "amara.okonkwo@gmail.com" -> "***@gmail.com"
    if email != "***@gmail.com":
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise AssertionError(f"✗ Email was not masked correctly! Expected '***@gmail.com', got: {email}")

    # Assert ssn is completely removed
    if ssn is not None:
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise AssertionError(f"✗ SSN was not removed! Got: {ssn}")

    # Assert phone is masked (should contain stars and not be the original '+234-803-456-7890')
    if phone == "+234-803-456-7890" or "*" not in str(phone):
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise AssertionError(f"✗ Phone was not masked correctly! Got: {phone}")

    # Assert date_of_birth is masked (should contain stars and not be the original '1990-03-15')
    if dob == "1990-03-15" or "*" not in str(dob):
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise AssertionError(f"✗ Date of Birth was not masked correctly! Got: {dob}")

    # Assert address.street is removed
    if street is not None:
        auvis_proc.terminate()
        fastapi_proc.terminate()
        raise AssertionError(f"✗ Address street was not removed! Got: {street}")

    print("✓ Masking assertions passed perfectly!")

    # 8. Trigger Graceful Shut down to flush audit logger to disk
    print("\n→ Initiating graceful shutdown of Auvis dev proxy...")
    auvis_proc.terminate()
    fastapi_proc.terminate()
    
    # Wait for processes to flush and clean up
    time.sleep(1.0)
    print("✓ Services shutdown successfully.")

    # 9. Verify Audit Log entry written
    print("\n→ Verifying Audit Log Entries...")
    if not os.path.exists(audit_log_path):
        raise AssertionError("✗ Audit log file was not written!")

    with open(audit_log_path, "r") as f:
        content = f.read().strip()

    if not content:
        raise AssertionError("✗ Audit log file was empty!")

    # Split by double newline to handle pretty-printed JSON blocks
    blocks = [b.strip() for b in content.split("\n\n") if b.strip()]
    if not blocks:
        raise AssertionError("✗ Audit log file had no valid blocks!")

    audit_entry = json.loads(blocks[0])
    print("  - Policies Applied recorded:", audit_entry.get("policies_applied"))
    print("  - Fields Masked recorded:", audit_entry.get("fields_masked"))
    print("  - Priority recorded:", audit_entry.get("priority"))
    
    if audit_entry.get("priority") != 10:
        raise AssertionError(f"✗ Expected priority to be 10, got: {audit_entry.get('priority')}")

    if "users-analyst-masked" not in audit_entry.get("policies_applied", []):
        raise AssertionError(f"✗ Expected 'users-analyst-masked' in logged policy names, got: {audit_entry.get('policies_applied')}")

    if "email" not in audit_entry.get("fields_masked", []):
        raise AssertionError(f"✗ Expected 'email' in fields_masked logs, got: {audit_entry.get('fields_masked')}")

    print("✓ Audit log assertions passed perfectly!")
    print("\n=======================================================")
    print("🎉 ALL END-TO-END PII MASKING TESTS PASSED SUCCESSFULLY!")
    print("=======================================================")

if __name__ == "__main__":
    try:
        run_e2e_test()
    except Exception as e:
        print(f"\n{e}")
        sys.exit(1)
