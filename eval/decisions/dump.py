"""Re-run the set through Jev keeping every answer, so the detail can be read rather than summarised."""
import json
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from evaluate import DATA, REASON_Q, SEARCH_Q, YESNO_Q
from jev_eval import decide

HERE = Path(__file__).parent
out = {}

with ThreadPoolExecutor(max_workers=8) as pool:
    cases = json.loads((DATA / "reasons.json").read_text())
    answers = list(pool.map(lambda c: decide({"body": c["text"]}, REASON_Q), cases))
    out["reason"] = [
        {**c, "got": a["reason"]["choice"], "confidence": a["reason"].get("confidence"),
         "probabilities": {k: round(v, 3) for k, v in (a["reason"].get("probabilities") or {}).items()}}
        for c, a in zip(cases, answers)
    ]

    cases = json.loads((DATA / "questionnaire.json").read_text())
    answers = list(pool.map(lambda c: decide({"reply": c["reply"]}, YESNO_Q), cases))
    out["questionnaire"] = [
        {**c, "got": {k: ("yes" if float(a[k]["noul"]) >= 0.5 else "no") for k in c["want"]},
         "p": {k: round(float(a[k]["noul"]), 3) for k in c["want"]}}
        for c, a in zip(cases, answers)
    ]

    cases = json.loads((DATA / "search.json").read_text())
    answers = list(pool.map(lambda c: decide({"query": c["query"]}, SEARCH_Q), cases))
    out["search"] = [
        {**c, "got": {"state": a["state"]["choice"], "reason": a["reason"]["choice"],
                      "overdue": "yes" if float(a["overdue"]["noul"]) >= 0.5 else "no"},
         "confidence": {"state": a["state"].get("confidence"), "reason": a["reason"].get("confidence"),
                        "overdue": round(float(a["overdue"]["noul"]), 3)}}
        for c, a in zip(cases, answers)
    ]

(HERE / "jev-answers.json").write_text(json.dumps(out, indent=1, ensure_ascii=False))
print("wrote jev-answers.json:", {k: len(v) for k, v in out.items()})
