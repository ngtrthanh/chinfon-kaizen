# MIS security acceptance — Chinfon Kaizen 360 (ASVS 5.0 Level 2)

**Decision:** NOT ACCEPTED / NOT YET VERIFIED. This checklist is a management entrypoint; full evaluation must cover the 253 L1+L2 requirements in `criteria-5.0.0-L2.json`. Do not present these 12 scenarios as full ASVS compliance.

## Roles

- Business owner (Kaizen/HR): data categories, financial formula, user roles, UAT, retention.
- MIS: network/endpoint controls, identity provisioning, code review coordination, restore drill, monitoring.
- Independent reviewer: exploit-oriented testing, evidence, retest and sign-off.
- DGD: approve rollout scope only after recorded risk assessment and accountable sign-offs.

## Twelve checks for a non-specialist review meeting

| ID | Scenario (run on TEST data) | Expected result | Evidence | Decision |
|---|---|---|---|---|
| M01 | Employee A changes URL ID to view Employee B record | Denied; no data leakage | request/response; user roles | PENDING |
| M02 | Manager of P81 opens different department data | Denied by backend and SQL scope | API test; DB view | PENDING |
| M03 | Council member opens CONFIDENTIAL initiative | Denied without even revealing existence | 404/access log | PENDING |
| M04 | Pending user uses last 5 digits of mobile number | Login fails | integration test + runtime screenshot | PENDING |
| M05 | Server operator issues random invite and user completes initial login | 30-minute credential; forced password change + MFA; no plaintext DB storage | CLI/test log with secrets redacted | PENDING |
| M06 | Unenrolled MFA user requests payroll export or admin endpoint | Denied/redirected before protected operation | request/response | PENDING |
| M07 | User submits POST without CSRF or with another user's token | Denied | test log | PENDING |
| M08 | User uploads disguised executable or macro Office package | Rejected; unsupported files refused | fixture and test log | PENDING |
| M09 | Attacker downloads attachment outside authorized record | Denied; no attachment contents | request/response | PENDING |
| M10 | Staff browser connects over internal LAN | HTTPS, valid certificate, Secure cookie, no direct plaintext port | browser/proxy/firewall config | PENDING |
| M11 | Restore DB, files and MFA key in isolated machine | Data consistent; accounts and MFA behave as expected | signed restore report and measured RPO/RTO | PENDING |
| M12 | Patch toolchain, dependencies and rerun scan | No reachable high/critical vulnerability; CI evidence | build provenance, govulncheck artifact | PENDING |

These are proposed verification scenarios, not recorded PASS results.

## Required protocol for every ASVS requirement

1. Classify as applicable or not applicable. A not-applicable decision needs written rationale.
2. Identify implemented control and code/config reference.
3. Test with defined inputs and expected output in isolated staging.
4. Attach command output, redacted request/response, log or screenshot and commit SHA.
5. Independent MIS/security verifier records PASS or FAIL, date, identity, and remediation reference.
6. Retest every fixed FAIL. Do not convert code-reviewed or CI-green controls to PASS without verification.

## Minimum rollout conditions

- **No production personal records** until P0 credentials, role/scope, secure transport, administrative access, and recovery are verified.
- **No payroll authority** until reward formula and HR export are validated by business/Finance.
- **No public whistleblower intake** before privacy, protection, and sensitive-case handling are approved.
- Separate app evaluation from hosting/organizational security; ASVS is not the whole information security program.

## How MIS obtains the complete checklist

```bash
python3 tools/asvs_report.py --csv asvs-review.csv
```

The CSV covers 253 official requirements including description, L1/L2 level, verification result, evidence links, reviewer and N/A rationale. Unscored controls remain NOT_TESTED. Reference: https://github.com/OWASP/ASVS/releases/tag/v5.0.0_release

## Declaration

Developer: ________   MIS: ________   Independent tester: ________
Business owner: ________   DGD acceptance decision: ________
Reviewed commit: ________   Date: ________   Critical unresolved findings: ________
