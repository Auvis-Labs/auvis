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

def dump_proc_logs(auvis_proc, fastapi_proc):
    print("\n→ [DEBUG] Collecting Auvis Gateway and Upstream Stderr Logs...")
    time.sleep(0.5)  # allow logs to flush
    
    # Dump Auvis logs
    if auvis_proc:
        try:
            import fcntl
            fd = auvis_proc.stderr.fileno()
            fl = fcntl.fcntl(fd, fcntl.F_GETFL)
            fcntl.fcntl(fd, fcntl.F_SETFL, fl | os.O_NONBLOCK)
            err_logs = auvis_proc.stderr.read().decode()
            if err_logs:
                print("\n================== AUVIS GATEWAY LOGS ==================")
                print(err_logs)
                print("========================================================")
        except Exception as e:
            print(f"(could not read proxy logs: {e})")

    # Dump FastAPI logs
    if fastapi_proc:
        try:
            import fcntl
            fd = fastapi_proc.stderr.fileno()
            fl = fcntl.fcntl(fd, fcntl.F_GETFL)
            fcntl.fcntl(fd, fcntl.F_SETFL, fl | os.O_NONBLOCK)
            err_logs = fastapi_proc.stderr.read().decode()
            if err_logs:
                print("\n================== FASTAPI UPSTREAM LOGS ==================")
                print(err_logs)
                print("===========================================================")
        except Exception as e:
            print(f"(could not read upstream logs: {e})")


def run_e2e_llm_test():
    print("→ Initializing End-to-End LLM Interceptor Test...")

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

    fastapi_proc = None
    auvis_proc = None

    try:
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
            raise RuntimeError("✗ Failed to start FastAPI mock upstream server")
        print("✓ Mock upstream server is healthy and ready.")

        # 3. Start Auvis Dev Proxy on port 8080
        print("→ Starting Auvis dev gateway on port 8080...")
        env = os.environ.copy()
        env["AUVIS_UPSTREAM"] = "http://localhost:8000"
        env["AUVIS_PORT"] = "8080"
        env["AUVIS_AUDIT_FILE"] = audit_log_path

        # Run the compiled binary
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
                # Hitting /llm/chat without auth should return 401 Unauthorized, proving proxy is listening
                with urllib.request.urlopen("http://localhost:8080/llm/chat") as r:
                    pass
            except urllib.error.HTTPError as e:
                if e.code == 401:
                    ready = True
                    break
            except Exception:
                time.sleep(0.1)

        if not ready:
            raise RuntimeError("✗ Failed to start Auvis dev proxy server")
        print("✓ Auvis dev proxy is healthy and ready.")

        # 5. Generate signed JWT Analyst Token
        print("→ Generating JWT Analyst token...")
        token = create_access_token(user_id="usr_102", role="analyst")
        print(f"✓ Token generated: {token[:20]}...")

        # 6. Send LLM Chat request containing email and SSN PII to the proxy
        print("→ Sending outbound LLM chat request to proxy on http://localhost:8080/llm/chat...")
        payload = {
            "messages": [
                {"role": "user", "content": "Hello, my email is john.doe@example.com and SSN is 123-45-6789."}
            ]
        }
        req_body = json.dumps(payload).encode("utf-8")
        
        req = urllib.request.Request(
            "http://localhost:8080/llm/chat",
            data=req_body,
            headers={
                "Authorization": f"Bearer {token}",
                "X-Auvis-Upstream": "http://localhost:8000",
                "Content-Type": "application/json"
            },
            method="POST"
        )

        try:
            with urllib.request.urlopen(req) as response:
                status_code = response.status
                response_bytes = response.read()
        except urllib.error.HTTPError as e:
            status_code = e.code
            response_bytes = e.read()
        except Exception as e:
            raise RuntimeError(f"✗ Network error connecting to proxy: {e}")

        print(f"✓ Proxy responded with HTTP status code: {status_code}")

        if status_code != 200:
            raise AssertionError(f"✗ Expected HTTP 200 from proxy, got {status_code}. Response: {response_bytes.decode()}")

        # 7. Assert Response Rehydration on Client Side
        response_data = json.loads(response_bytes.decode())
        assistant_content = response_data["choices"][0]["message"]["content"]
        print(f"\n→ Assistant Response received by client:\n  \"{assistant_content}\"")
        
        # The client response MUST be rehydrated back to the original email and SSN
        if "john.doe@example.com" not in assistant_content or "123-45-6789" not in assistant_content:
            raise AssertionError("✗ Response was not rehydrated correctly! Original PII values are missing from response.")
        
        if "[EMAIL_1]" in assistant_content or "[SSN_1]" in assistant_content:
            raise AssertionError("✗ Response contained raw placeholder tokens instead of rehydrated values!")
            
        print("✓ Client-side response rehydration verified successfully!")

        # 8. Assert Masking on Upstream Side
        print("\n→ Verifying that mock upstream received masked request body...")
        try:
            with urllib.request.urlopen("http://localhost:8000/llm/last_request") as r:
                last_req_data = json.loads(r.read().decode())
        except Exception as e:
            raise AssertionError(f"✗ Failed to query upstream for last request data: {e}")

        last_request_body = last_req_data.get("last_request")
        if not last_request_body:
            raise AssertionError("✗ Upstream did not record the last request!")

        upstream_content = last_request_body["messages"][0]["content"]
        print(f"  - Request body received by upstream FastAPI: \"{upstream_content}\"")

        if "john.doe@example.com" in upstream_content or "123-45-6789" in upstream_content:
            raise AssertionError("✗ Outbound masking FAILED! Upstream received raw PII values!")

        if "[EMAIL_1]" not in upstream_content or "[SSN_1]" not in upstream_content:
            raise AssertionError("✗ Masking placeholders were not correctly structured in outbound request!")

        print("✓ Outbound masking verified successfully!")

        # 9. Trigger Graceful Shut down to flush audit logger to disk
        print("\n→ Initiating graceful shutdown of Auvis dev proxy...")
        auvis_proc.terminate()
        fastapi_proc.terminate()
        
        # Wait for processes to flush and clean up
        time.sleep(1.0)
        print("✓ Services shutdown successfully.")

        # 10. Verify Audit Log entry written
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
        
        if audit_entry.get("priority") != 100:
            raise AssertionError(f"✗ Expected priority to be 100, got: {audit_entry.get('priority')}")

        if "llm-pii-intercept" not in audit_entry.get("policies_applied", []):
            raise AssertionError(f"✗ Expected 'llm-pii-intercept' in logged policy names, got: {audit_entry.get('policies_applied')}")

        # The field masked recorded is what got masked during request intercept (EMAIL, SSN)
        fields_masked = audit_entry.get("fields_masked", [])
        if "EMAIL" not in fields_masked or "SSN" not in fields_masked:
            raise AssertionError(f"✗ Expected 'EMAIL' and 'SSN' in fields_masked logs, got: {fields_masked}")

        print("✓ Audit log assertions passed perfectly!")
        print("\n=======================================================")
        print("🎉 ALL END-TO-END LLM INTERCEPTOR TESTS PASSED!")
        print("=======================================================")

    except Exception as e:
        dump_proc_logs(auvis_proc, fastapi_proc)
        if auvis_proc:
            try:
                auvis_proc.terminate()
            except Exception:
                pass
        if fastapi_proc:
            try:
                fastapi_proc.terminate()
            except Exception:
                pass
        raise e

if __name__ == "__main__":
    try:
        run_e2e_llm_test()
    except Exception as e:
        print(f"\n{e}")
        sys.exit(1)
