import os
import sys
import shutil
import subprocess

def run_e2e_validate_test():
    print("→ Initializing End-to-End CLI Validate Test...")

    # Define paths
    workspace_dir = "/Users/Cridiv/Documents/Auvis"
    temp_dir = os.path.join(workspace_dir, "test-server", "temp_malformed_policies")

    # Clean existing temporary directory if any
    if os.path.exists(temp_dir):
        shutil.rmtree(temp_dir)
        print("✓ Cleaned existing temporary malformed policies directory.")
    os.makedirs(temp_dir)
    print("✓ Created fresh temporary malformed policies directory.")

    # 1. Define 10 distinct intentionally malformed policy contents
    malformed_policies = {
        "err1_missing_name.yaml": """
policies:
  - match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: mask
""",
        "err2_duplicate_name.yaml": """
policies:
  - name: duplicate-policy-name
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: mask
  - name: duplicate-policy-name
    match:
      roles: [admin]
      resource: /users
    rules:
      - field: "email"
        action: allow
""",
        "err3_invalid_action.yaml": """
policies:
  - name: invalid-action-policy
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: scram
""",
        "err4_invalid_tag.yaml": """
policies:
  - name: invalid-tag-policy
    compliance_tags: [GDPR_SUPER]
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: mask
""",
        "err5_resource_slash.yaml": """
policies:
  - name: resource-slash-policy
    match:
      roles: [analyst]
      resource: users
    rules:
      - field: "email"
        action: mask
""",
        "err6_missing_resource.yaml": """
policies:
  - name: missing-resource-policy
    match:
      roles: [analyst]
    rules:
      - field: "email"
        action: mask
""",
        "err7_structural_conflict.yaml": """
policies:
  - name: structural-policy-1
    priority: 10
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: mask
  - name: structural-policy-2
    priority: 10
    match:
      roles: [analyst]
      resource: /users
    rules:
      - field: "email"
        action: redact
""",
        "err8_empty_roles.yaml": """
policies:
  - name: empty-roles-policy
    match:
      roles: []
      resource: /users
    rules:
      - field: "email"
        action: mask
""",
        "err9_empty_rules.yaml": """
policies:
  - name: empty-rules-policy
    match:
      roles: [analyst]
      resource: /users
    rules: []
""",
        "err10_missing_rule_field.yaml": """
policies:
  - name: missing-field-policy
    match:
      roles: [analyst]
      resource: /users
    rules:
      - action: mask
"""
    }

    # Write the 10 malformed policies to the temporary directory
    print("\n→ Writing 10 malformed policy files to disk...")
    for filename, content in malformed_policies.items():
        filepath = os.path.join(temp_dir, filename)
        with open(filepath, "w") as f:
            f.write(content.strip())
        print(f"  - Wrote {filename}")

    # 2. Run the auvis validate command against the temporary directory
    print("\n→ Executing './auvis validate' against the malformed policies...")
    cmd = [
        "./auvis", "validate",
        "--policies", temp_dir,
        "--config", "auvis.yaml"
    ]
    
    result = subprocess.run(
        cmd,
        cwd=os.path.join(workspace_dir, "auvis-infra"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )

    # Clean up files immediately to keep filesystem clean
    print("\n→ Cleaning up temporary policies directory...")
    shutil.rmtree(temp_dir)
    print("✓ Cleaned up temporary files successfully.")

    # 3. Assert validation failure and analyze diagnostic output
    print(f"\n→ Validating Exit Code: {result.returncode} (Expected: 1)")
    if result.returncode != 1:
        print("\n=== STDOUT ===")
        print(result.stdout)
        print("=== STDERR ===")
        print(result.stderr)
        raise AssertionError(f"✗ Expected validation command to exit with code 1, got: {result.returncode}")
    print("✓ Exit code 1 verified successfully (Command correctly aborted).")

    print("\n→ Analyzing diagnostic error messages...")
    output_text = result.stdout + "\n" + result.stderr

    # Define the 10 expected error substrings
    expected_errors = [
        "every policy must have a name",
        "duplicate policy name — already defined in",
        "'scram' is not a valid action",
        "invalid compliance tag \"GDPR_SUPER\"",
        "must start with /",
        "resource path is required",
        "structural conflict: duplicate priority 10",
        "at least one role is required",
        "at least one rule is required",
        "rule field is required"
    ]

    print("\n=== CAPTURED VALIDATION DIAGNOSTICS ===")
    print(result.stdout)
    print("========================================")

    missing_errors = []
    for err in expected_errors:
        if err not in output_text:
            missing_errors.append(err)
            print(f"❌ Missing expected error: \"{err}\"")
        else:
            print(f"✓ Caught expected error: \"{err}\"")

    if missing_errors:
        raise AssertionError(f"✗ Validation command failed to catch all expected errors! Missing errors: {missing_errors}")

    print("\n=======================================================")
    print("🎉 ALL 10 POLICY VALIDATION ERRORS CAUGHT SUCCESSFULLY!")
    print("=======================================================")

if __name__ == "__main__":
    try:
        run_e2e_validate_test()
    except Exception as e:
        print(f"\n{e}")
        sys.exit(1)
