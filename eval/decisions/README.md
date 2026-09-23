# Typed-decision evaluation

The labelled set behind ADR 0024 and the two runners that score a model against it. The set is built
from this system's own vocabulary: the four reasons from `domain/questionnaire.go`, the yes and no
questions of `domain/questionnaires/UNAUTHORISED.json`, and the states of `domain/state.go`.

| File | What |
| --- | --- |
| `reasons.json` | 72 complaints, balanced across the four reasons, in English, Hungarian and a code-switched pair; a `hard` band of 20 states a distracting reason before the real one |
| `questionnaire.json` | 40 customer replies carrying 113 labelled yes and no answers; bands for plain, `indirect` (the answer is implied) and `hard` (negation, litotes, run-on) |
| `search.json` | 50 analyst queries over 69 labelled filter fields, including terse idiom and Hungarian |
| `evaluate.py` | Runs the set against local Laya checkpoints; writes `results-laya.json` |
| `jev_eval.py` | Runs the same set against the hosted model; writes `results-jev.json` |
| `dump.py` | Keeps every answer with its probabilities, for reading rather than summarising |

The labels are the author's, and the prose is written rather than collected: no corpus of real
customer language exists for this system, so the set measures recovery of an intent that was
encoded. Any number from it belongs in the thesis with that sentence attached.

## Running

```sh
python3 -m venv .venv && .venv/bin/pip install laya      # local model, optional
OPENROUTER_API_KEY=... .venv/bin/python jev_eval.py       # hosted model
```

Two things this machine needed: install CPU torch from `download.pytorch.org/whl/cpu` first,
because pip's CUDA stack stalls here, and fetch checkpoints with curl rather than
`huggingface_hub`, whose client hangs where curl reaches 12 MB/s. `jev_eval.py` shells out to curl
for the same reason.
