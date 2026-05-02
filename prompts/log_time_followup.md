---
name: log_time_followup
title: Prepare Time Entry Follow-Up
description: Draft a follow-up message after logging work in Redmine.
arguments:
  - name: issue_id
    description: Numeric Redmine issue ID linked to the time entry.
    required: true
  - name: hours
    description: Hours that were logged.
    required: true
---
Write a concise update for Redmine issue #{{.issue_id}} confirming that {{.hours}} hours were logged.

Include:
1. What changed.
2. Any remaining work.
3. Whether another time entry should be expected.
