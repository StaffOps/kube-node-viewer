# Design: <feature>

> Implements [requirements](requirements.md).

## Current state

<How it works today, with file:line references.>

## Target state

<How it will work. Diagram if the flow crosses packages
(client → model → UI, or pricing).>

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/...` | <change> |

## Decisions

### D1: <short title of the choice>

**Choice**: <one sentence>.

**Justification, strongest first**:
1. <reason>

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| <cost> | <reason> |

**When this would be wrong**: <signal that should reopen it>.

**Reversal action**: <concrete steps to undo, and their cost>.

**Alternatives discarded**: <alternative> — <why>.

## Invariants

- <rule the change must never break, e.g. "requests, never usage">

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| <API down, missing label, no pricing> | <behavior> | <symptom> |

## Test plan

- <unit tests in pkg/model, manual check against a named cluster>
