# Implementation Plan: Inbox provider actions and production QA

## Overview

Repair the Inbox controls that currently render without behavior, using Gmail's supported message-label mutations. Then deploy and verify the repair plus the safe, high-value parts of the live Warmbly workspace.

## Architecture Decisions

- Treat archive and delete as Gmail label changes, never as permanent deletion of local rows.
- Make every thread action explicitly org-authorized and return an error for unsupported providers instead of pretending it succeeded.
- Use the existing test conversation for production verification; do not send or remove real customer mail.

## Task List

### Phase 1: Inbox action slice

- [x] Add Gmail client operations for archive, trash, and unread.
- [x] Add an org-scoped Unibox thread-action API that resolves the selected thread and calls the provider.
- [x] Add the frontend client/mutation and wire desktop and mobile thread controls, with delete confirmation.

### Checkpoint: Build verification

- [x] Run focused Go checks and frontend type-check.

### Phase 2: Deployment and QA

- [x] Deploy the action repair and validate delete state with the existing test thread.
- [x] Perform a safe production QA pass and report working, failing, and intentionally untested features.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Inbox action affects a real message | High | Use only the known test thread. |
| Gmail's remote state and local cache differ briefly | Medium | Invalidate lists/overview and wait for sync before declaring success. |
| Other email providers lack an equivalent implementation | Medium | Return a clear unsupported response until their provider-specific action is implemented. |
