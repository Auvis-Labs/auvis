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


def run_e2e_reload_test():
    print("→ Initializing End-to-End Policy Hot-Reload Test...")

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
    policies_dir = os.path.join(workspace_dir, "auvis-infra", "policies")
    temp_policy_path = os.path.join(policies_dir, "temp_reload.yaml")
    audit_log_path = os.path.join(workspace_dir, "audit.jsonl")

    # Clean existing audit log and temporary policies
    if os.path.exists(audit_log_path):
        os.remove(audit_log_path)
        print("✓ Cleaned existing audit log.")
        
    if os.path.exists(temp_policy_path):
        os.remove(temp_policy_path)
        print("✓ Cleaned existing temporary policy file.")

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
            raise RuntimeError("✗ Failed to start Auvis dev proxy server")
        print("✓ Auvis dev proxy is healthy and ready.")

        # 5. Generate signed JWT Analyst Token
        print("→ Generating JWT Analyst token...")
        token = create_access_token(user_id="usr_101", role="analyst")
        print(f"✓ Token generated: {token[:20]}...")

        # 6. Request 1: Base request with default policy configuration (email should be MASKED)
        print("\n→ Request 1: Testing default policy masking...")
        req = urllib.request.Request(
            "http://localhost:8080/users",
            headers={
                "Authorization": f"Bearer {token}",
                "X-Auvis-Upstream": "http://localhost:8000"
            }
        )
        with urllib.request.urlopen(req) as r:
            res_data = json.loads(r.read().decode())
        
        email_val = res_data["data"][0]["email"]
        print(f"  - Received email: \"{email_val}\"")
        if email_val != "***@gmail.com":
            raise AssertionError(f"✗ Expected masked email '***@gmail.com', got: '{email_val}'")
        print("✓ Default policy masking verified successfully.")

        # 7. Dynamically write high-priority policy file (temp_reload.yaml) to active directory
        print("\n→ Dynamically creating higher-priority override policy 'temp_reload.yaml'...")
        reload_policy = {
            "policies": [
                {
                    "name": "reload-test-policy",
                    "priority": 999,
                    "match": {
                        "roles": ["analyst"],
                        "resource": "/users"
                    },
                    "rules": [
                        {
                            "field": "email",
                            "action": "redact"
                        }
                    ],
                    "response": {
                        "data_path": "data",
                        "array": True
                    }
                }
            ]
        }
        
        import yaml
        with open(temp_policy_path, "w") as f:
            yaml.safe_dump(reload_policy, f)
            
        print("✓ temp_reload.yaml successfully written to disk. Waiting 1.2 seconds for hot-reload...")
        time.sleep(1.2) # allow watcher to capture event and reload

        # 8. Request 2: Send request again (email should now be REDACTED)
        print("\n→ Request 2: Testing hot-reloaded override policy...")
        with urllib.request.urlopen(req) as r:
            res_data = json.loads(r.read().decode())
            
        email_val = res_data["data"][0]["email"]
        print(f"  - Received email: \"{email_val}\"")
        if email_val != "[REDACTED]":
            raise AssertionError(f"✗ Expected redacted email '[REDACTED]', got: '{email_val}'")
        print("✓ Policy hot-reload loaded and applied successfully!")

        # 9. Dynamically delete the temporary policy file from disk
        print("\n→ Dynamically deleting override policy file 'temp_reload.yaml'...")
        os.remove(temp_policy_path)
        print("✓ temp_reload.yaml successfully deleted. Waiting 1.2 seconds for hot-reload unload...")
        time.sleep(1.2) # allow watcher to capture delete event

        # 10. Request 3: Send request again (email should revert back to MASKED)
        print("\n→ Request 3: Testing hot-reload policy unload...")
        with urllib.request.urlopen(req) as r:
            res_data = json.loads(r.read().decode())
            
        email_val = res_data["data"][0]["email"]
        print(f"  - Received email: \"{email_val}\"")
        if email_val != "***@gmail.com":
            raise AssertionError(f"✗ Expected email to revert to masked '***@gmail.com', got: '{email_val}'")
        print("✓ Policy hot-reload unloaded successfully (reverted back to default policy)!")

        # 11. Trigger Graceful Shut down to flush audit logger to disk
        print("\n→ Initiating graceful shutdown of Auvis dev proxy...")
        auvis_proc.terminate()
        fastapi_proc.terminate()
        
        # Wait for processes to flush and clean up
        time.sleep(1.0)
        print("✓ Services shutdown successfully.")

        # 12. Verify Audit Log entry written
        print("\n→ Verifying Audit Log Entries...")
        if not os.path.exists(audit_log_path):
            raise AssertionError("✗ Audit log file was not written!")

        with open(audit_log_path, "r") as f:
            content = f.read().strip()

        if not content:
            raise AssertionError("✗ Audit log file was empty!")

        # Split by double newline to handle pretty-printed JSON blocks
        blocks = [b.strip() for b in content.split("\n\n") if b.strip()]
        if len(blocks) < 3:
            raise AssertionError(f"✗ Expected at least 3 log entries, got: {len(blocks)}")

        # Block 0: First request (should not contain reload-test-policy)
        entry_0 = json.loads(blocks[0])
        print("  - Entry 1 Policies Applied:", entry_0.get("policies_applied"))
        if "reload-test-policy" in entry_0.get("policies_applied", []):
            raise AssertionError("✗ Expected reload-test-policy NOT to be applied in entry 1")

        # Block 1: Second request (MUST contain reload-test-policy)
        entry_1 = json.loads(blocks[1])
        print("  - Entry 2 Policies Applied:", entry_1.get("policies_applied"))
        print("  - Entry 2 Priority:", entry_1.get("priority"))
        if "reload-test-policy" not in entry_1.get("policies_applied", []):
            raise AssertionError("✗ Expected reload-test-policy to be applied in entry 2")
        if entry_1.get("priority") != 999:
            raise AssertionError(f"✗ Expected priority to be 999 in entry 2, got: {entry_1.get('priority')}")

        # Block 2: Third request (should not contain reload-test-policy)
        entry_2 = json.loads(blocks[2])
        print("  - Entry 3 Policies Applied:", entry_2.get("policies_applied"))
        if "reload-test-policy" in entry_2.get("policies_applied", []):
            raise AssertionError("✗ Expected reload-test-policy NOT to be applied in entry 3")

        print("✓ Audit log assertions passed perfectly!")
        print("\n=======================================================")
        print("🎉 ALL POLICY HOT-RELOAD INTEGRATION TESTS PASSED!")
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
        if os.path.exists(temp_policy_path):
            try:
                os.remove(temp_policy_path)
            except Exception:
                pass
        raise e

if __name__ == "__main__":
    try:
        run_e2e_reload_test()
    except Exception as e:
        print(f"\n{e}")
        sys.exit(1)
