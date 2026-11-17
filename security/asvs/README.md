# Chinfon Kaizen — OWASP ASVS 5.0.0 Level 2 verification

This is an **implementation and audit program**, not a certificate or assertion of compliance.

## Baseline and license
- Canonical OWASP release: https://github.com/OWASP/ASVS/releases/tag/v5.0.0_release
- Pinned reference: `criteria-5.0.0-L2.json` (253 requirements: 70 L1 + 183 L2).
- Source copyright: OWASP ASVS contributors. Source licensed CC BY-SA 4.0. Attribution and unchanged references are maintained.
- Status and evidence are stored separately: `assessment.json`. Missing status = NOT_TESTED.
- Generated MIS spreadsheet: `python3 tools/asvs_report.py --csv asvs-review.csv`.

## Acceptance
For each applicable requirement, an independent reviewer must document:
1. The threat and the role/data in scope.
2. Test steps and test credentials in a nonproduction environment.
3. Expected and actual behavior, test log, version/commit and date.
4. PASS/FAIL with durable evidence; NOT_APPLICABLE requires rationale.
5. Reviewer identity, remediation ticket, retest evidence.

The tool refuses PASS without evidence but **does not validate the evidence itself**. No requirement is currently recorded as PASS.

## Priority
- P0: random bootstrap token; access-control fail-closed; HTTPS proxy and secure cookies; independent IDOR tests across department and CONFIDENTIAL; sensitive file/storage permissions.
- P1: content-type signatures and Office package checks; automated vulnerability checks and regression CI; restore drill including mfa.key; MFA reset verification.
- P2: dependency SBOM, privacy retention and deletion workflows, full ASVS evidence mapping, secure deployment and source handover.

Remaining gates: external security review, SSO vs local auth ownership, data-processing inventory/retention, backup offsite with restore proof, protected TLS deployment, monitoring and incident response. These cannot be declared passed by static code review.
