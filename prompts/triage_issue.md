---
name: triage_issue
title: Triage Redmine Issue
description: Generate a structured triage plan for a Redmine issue before making changes.
arguments:
  - name: issue_id
    description: Numeric Redmine issue ID to inspect.
    required: true
  - name: project_hint
    description: Optional project identifier to mention in the analysis.
---
Review Redmine issue #{{.issue_id}}{{if .project_hint}} in project {{.project_hint}}{{end}}.

Produce:
1. A short restatement of the problem.
2. Unknowns or blockers that need validation.
3. The smallest safe implementation plan.
4. The MCP tools that should be called first.
