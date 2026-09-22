# 0024: An external typed-decision model proposes, a person disposes

**Status:** accepted, 2026-09-22; builds on ADR 0012, 0015 and 0016

## Context

Three places in the workbench turn a person's prose into a value the domain already has a type for.
An intake describes a problem and someone picks one of four reasons. A customer answers a
questionnaire in a paragraph and an analyst records typed answers. An analyst wants a list and
types a sentence rather than setting three filters. Each is the same shape: unstructured text in,
a value from a closed vocabulary out.

Typed decision models answer exactly that shape. They take a state and a set of questions whose
answers are constrained (one of these options, this ordered scale, yes or no) and return the value
with a probability, in one forward pass and without generating text, so there is nothing to parse
and no free-form output to police. Two were measured against a labelled set built from this
system's own vocabulary: 72 reason cases, 113 questionnaire answers and 69 search fields, each with
a deliberately hard band and Hungarian throughout, since both seeded tenants are Hungarian banks.

Laya, the open-source model, runs locally in 8 to 17 ms on a consumer GPU and is not accurate
enough: 0.64 on the four-way reason task and 0.61 on a binary questionnaire task, with an expected
calibration error between 0.21 and 0.60, so its confidence cannot be trusted either. Jev, the
hosted model, scored 0.99, 0.97 and 0.93 on the same three sets with an ECE of 0.011, at 574 ms and
$0.0000216 per call. The gap is too large to argue with, and the calibration gap matters as much as
the accuracy: a model whose confidence is meaningful can be asked to abstain.

## Decision

- The model sits behind a port, `Decisions`, in the application layer, next to `BankingCore`
  (ADR 0015) and for the same reason: it is someone else's system, it can be slow, it can be down,
  and the domain must not know which one it is. Tests and local development run a deterministic
  fake; the shipped adapter calls the hosted model.
- **It proposes a value a person confirms. It never decides.** No transition, no clock, no posting
  and no letter may depend on it. Every use is a field that arrives filled in and can be changed
  before anything is recorded, and the recorded value is always the one the person left.
- When a proposal is accepted, what is recorded on the event is the accepted value plus the fact
  that it was proposed, by which model version, at what probability. The audit trail therefore
  stays a record of what a person did, with the machine's contribution visible beside it.
- The port is optional. With no adapter configured every one of these features degrades to what
  the system does today: an empty field the analyst fills. The engine does not acquire a dependency
  it cannot run without, which is also what keeps ADR 0005's claim about Postgres honest.
- Confidence gates the proposal. Below a threshold the field is left empty rather than filled with
  a guess, because a wrong pre-filled answer costs more attention than an empty one.
- The three uses, in the order they will be built: the search sentence, which touches no stored
  data and whose errors are visible as filters; the reason at intake, which needs a description
  field the contract does not have yet; and the questionnaire, which needs somewhere to put the
  customer's reply.

## Consequences

- Easier: three transcription jobs get faster without the domain learning anything new; the same
  port serves all three; with no adapter the system behaves exactly as it does now, so the feature
  is reversible in configuration rather than in code.
- Harder: customer prose leaves the institution on every call, which is a data protection question
  that a bank answers before a benchmark does, and it is the reason the port exists and the reason
  a self-hosted adapter must stay possible even though the open model is not good enough today.
  A second external dependency joins the banking core in the availability budget, though only on
  paths that degrade to an empty field. And the evaluation now has to be maintained: the labelled
  set is the only evidence the threshold is right, and it is synthetic, written by the author, so
  it measures recovery of an intent that was encoded rather than real customer language.
