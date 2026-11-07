Naveed Muhammad Muneeb
CM903S

# Thesis 1: summary of the semester

Theme: Implementing Business Logic using Modern Technologies (Automated Fraud Dispute Resolution Engine).
Working title: The Customer Is Always Right, Except When It's Not: Finite State Machines for Payment Dispute
Resolution and First-Party Fraud Detection.

Repository: https://github.com/muneeebnaveeed/thesis-dispute-engine

## What was planned

The proposal set out a dispute resolution engine for a card-issuing bank in six phases: research and planning,
backend core, backend advanced, frontend, integration and hardening, then evaluation and writing. The plan for
this semester was the first three phases; the frontend and integration were planned for the winter break and
early spring.

## What was done

**Research and planning (September).** A review of the regulations the engine follows: PSD2 card payments, SEPA
direct debits, and the US Regulations E and Z as a contrast. From it came the central design decision: one state
machine whose regime decides which parts of it apply, instead of a separate workflow per regime.

**The engine (September and October).** A Go service on PostgreSQL, built further than the semester's plan.
Every part has a written design decision (28 records in `docs/adr`):

- the dispute lifecycle as one state machine for four regimes, with tests that pin each regime's reachable
  states and require a named scenario for every allowed transition;
- regulatory deadlines computed in each bank's own business-day calendar;
- a double-entry ledger for refunds and provisional credit, with a test showing that every path to a closed
  dispute leaves the suspense account at zero;
- a simulated banking core that can decline, time out or answer slowly, called so that a retry cannot pay twice;
- the customer questionnaire, rule-based fraud scoring with explained signals, and customer notices as email
  and printable letters;
- several banks on one deployment, kept apart by the database's row-level security; sign-in per bank through
  Keycloak; and role and team based authorization written in Cedar.

**The analyst workbench.** A web application (TypeScript, TanStack Start) for analysts to work disputes. It was
planned for the next phase and built early.

**Quality and evaluation.** Every change went through a pull request with automated checks (83 pull requests
of my own). The latest committed evaluation run passed 189 test functions. It also measured latency under load
and recorded traces, metrics and logs.

**The thesis document.** A complete first draft in LaTeX, covering the introduction, background, requirements,
design, implementation, evaluation and conclusion. Each claim in it was checked against the code, the claims the
code contradicted were corrected, and the main text was cut to 30 pages.

## What remains for Thesis 2

- Polish the language and use the faculty title page and AI declaration.
- Close the remaining gaps the claim check found, and decide with my supervisor whether any part needs more
  depth or evaluation.
- Prepare the defence and the live demonstration.

Generative AI (Claude) was used for coding assistance and for drafting text. This is declared in the thesis.
