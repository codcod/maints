# Maintenance Issue Triage Checklist

Use this checklist to verify that a Jira maintenance issue contains all
required information before it can be accepted for scheduling.

---

## 1. Priority and Motivation
What is the priority and what is the reason/motivation for that priority?

The issue must state the chosen priority level (e.g. Critical, High, Medium,
Low) **and** explain why that priority was selected (business impact, number of
affected users, severity of the problem, etc.).

## 2. Customers Field
The Customers field must be populated, identifying which customer(s) or
accounts are affected.

## 3. Affected Software and Version
What software is affected, and which version?

The `affectedVersion` field in Jira must be set. The description or comments
should also clearly identify the software product and the exact version(s)
where the problem occurs.

## 4. Target Fix Version (as a comment)
Which version should the fix be applied to?

This must be provided as a **comment** on the issue (the squad completing the
fix will set the `fixVersion` field after analysis and scheduling). The comment
must identify the intended target version(s).

## 5. Relevant Logs
Relevant logs must be attached or included. This includes:
- Backend/server logs
- Web/frontend logs
- Mobile logs (if applicable)
- Network traffic logs between components (e.g. captured with Proxyman,
  Charles, or similar tools)

All logs should be filtered or annotated to highlight the section relevant to
the reported issue.
