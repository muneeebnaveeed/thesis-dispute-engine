"""Run the labelled sets against every local Laya checkpoint on the GPU.

Reports accuracy per use case, split by difficulty band and language, a confusion matrix for the
reason task, expected calibration error, and latency. Writes results.json beside this file.
"""

import json, statistics, sys, time
from collections import Counter, defaultdict
from pathlib import Path

import torch
from laya import Agent

HERE = Path(__file__).parent
DATA = HERE
# checkpoints are not in the repository: see README.md for fetching them
CHECKPOINTS = {"english": "models/laya", "multilingual": "models/laya-multilingual", "typed": "models/laya-typed"}

REASONS = {
    "UNAUTHORISED": "the customer says they did not make or authorise the payment at all",
    "NOT_RECEIVED": "the customer paid but the goods or services never arrived, or arrived only in part",
    "DUPLICATE": "the same purchase was charged more than once",
    "AMOUNT_DIFFERS": "the customer made the purchase but was charged a different amount than agreed",
}
STATES = [
    "INITIATED", "INVESTIGATING", "QUESTIONNAIRE_SENT", "QUESTIONNAIRE_RECEIVED", "PROVISIONAL_CREDIT_ISSUED",
    "FAST_REFUND_ISSUED", "SEPA_NQA_REFUND_ISSUED", "CHARGEBACK_FILED", "CHARGEBACK_ACKNOWLEDGED",
    "EVIDENCE_SUBMITTED", "CHARGEBACK_WON", "CHARGEBACK_LOST", "FINAL_CREDIT_ISSUED",
    "PROVISIONAL_CREDIT_REVERSED", "CLOSED",
]
YESNO = {
    "recognise_merchant": "Does the customer recognise the merchant?",
    "card_in_possession": "Was the customer's card in their possession at the time?",
    "shared_credentials": "Has anyone else had access to the card or its details?",
    "prior_disputes_merchant": "Has the customer disputed a transaction with this merchant before?",
    "police_report": "Has the customer reported the card lost or stolen, or filed a police report?",
}

REASON_Q = {"reason": {"type": "choice", "instructions": "Why is the customer disputing this payment?", "criteria": REASONS}}
YESNO_Q = {key: {"type": "noul", "instructions": text} for key, text in YESNO.items()}
SEARCH_Q = {
    "state": {"type": "choice", "instructions": "Which dispute state is the analyst asking for?",
              "criteria": {**{s: s.lower().replace("_", " ") for s in STATES}, "any": "no particular state"}},
    "reason": {"type": "choice", "instructions": "Which dispute reason is the analyst asking for?",
               "criteria": {**REASONS, "any": "no particular reason"}},
    "overdue": {"type": "noul", "instructions": "Is the analyst asking only for disputes that are past a deadline?"},
}


def ece(pairs, bins=10):
    """Expected calibration error: |confidence - accuracy| averaged over confidence bins."""
    if not pairs:
        return float("nan")
    buckets = defaultdict(list)
    for confidence, correct in pairs:
        buckets[min(int(confidence * bins), bins - 1)].append((confidence, correct))
    return sum(
        len(items) / len(pairs) * abs(statistics.fmean(c for c, _ in items) - statistics.fmean(float(k) for _, k in items))
        for items in buckets.values()
    )


def latency(agent, questions, state, runs=20):
    agent.system_one(state, questions)
    out = []
    for _ in range(runs):
        t = time.perf_counter()
        agent.system_one(state, questions)
        out.append((time.perf_counter() - t) * 1000)
    return sorted(out)


def run(name, path):
    agent = Agent(str(HERE / path), device="cuda")
    result = {"checkpoint": name}

    lat1 = latency(agent, REASON_Q, {"body": "I did not make this payment."})
    lat5 = latency(agent, YESNO_Q, {"reply": "The card was with me and nobody else has it."})
    result["latency_ms"] = {"one_question_median": round(statistics.median(lat1), 1),
                            "five_questions_median": round(statistics.median(lat5), 1),
                            "one_question_p90": round(lat1[int(len(lat1) * 0.9)], 1)}

    # ---- reason
    cases = json.loads((DATA / "reasons.json").read_text())
    by_band, by_lang, confusion, calib = Counter(), Counter(), Counter(), []
    totals_band, totals_lang, hits = Counter(), Counter(), 0
    for case in cases:
        answer = agent.system_one({"body": case["text"]}, REASON_Q)["answers"]["reason"]
        got, ok = answer["choice"], answer["choice"] == case["reason"]
        hits += ok
        by_band[case["band"]] += ok; totals_band[case["band"]] += 1
        by_lang[case["lang"]] += ok; totals_lang[case["lang"]] += 1
        confusion[(case["reason"], got)] += 1
        calib.append((float(answer.get("confidence", 0)), ok))
    result["reason"] = {
        "accuracy": round(hits / len(cases), 3), "n": len(cases),
        "by_band": {b: f"{by_band[b]}/{totals_band[b]}" for b in totals_band},
        "by_lang": {l: f"{by_lang[l]}/{totals_lang[l]}" for l in totals_lang},
        "ece": round(ece(calib), 3),
        "confusion": {f"{want}->{got}": n for (want, got), n in sorted(confusion.items()) if want != got},
    }

    # ---- questionnaire
    cases = json.loads((DATA / "questionnaire.json").read_text())
    per_q, per_q_total, by_band, totals_band, calib = Counter(), Counter(), Counter(), Counter(), []
    hits = total = 0
    for case in cases:
        answers = agent.system_one({"reply": case["reply"]}, YESNO_Q)["answers"]
        for key, want in case["want"].items():
            probability = float(answers[key]["noul"])
            got = "yes" if probability >= 0.5 else "no"
            ok = got == want
            hits += ok; total += 1
            per_q[key] += ok; per_q_total[key] += 1
            by_band[case["band"]] += ok; totals_band[case["band"]] += 1
            calib.append((probability if got == "yes" else 1 - probability, ok))
    result["questionnaire"] = {
        "accuracy": round(hits / total, 3), "n": total,
        "per_question": {q: f"{per_q[q]}/{per_q_total[q]}" for q in per_q_total},
        "by_band": {b: f"{by_band[b]}/{totals_band[b]}" for b in totals_band},
        "ece": round(ece(calib), 3),
    }

    # ---- search
    cases = json.loads((DATA / "search.json").read_text())
    per_field, per_field_total, by_band, totals_band = Counter(), Counter(), Counter(), Counter()
    hits = total = exact = 0
    for case in cases:
        answers = agent.system_one({"query": case["query"]}, SEARCH_Q)["answers"]
        got = {"state": answers["state"]["choice"], "reason": answers["reason"]["choice"],
               "overdue": "yes" if float(answers["overdue"]["noul"]) >= 0.5 else "no"}
        all_ok = True
        for field, want in case["want"].items():
            ok = got[field] == want
            hits += ok; total += 1; all_ok &= ok
            per_field[field] += ok; per_field_total[field] += 1
            by_band[case["band"]] += ok; totals_band[case["band"]] += 1
        exact += all_ok
    result["search"] = {
        "field_accuracy": round(hits / total, 3), "n_fields": total,
        "query_exact": f"{exact}/{len(cases)}",
        "per_field": {f: f"{per_field[f]}/{per_field_total[f]}" for f in per_field_total},
        "by_band": {b: f"{by_band[b]}/{totals_band[b]}" for b in totals_band},
    }

    result["vram_gb"] = round(torch.cuda.max_memory_allocated() / 1e9, 2)
    del agent
    torch.cuda.empty_cache(); torch.cuda.reset_peak_memory_stats()
    return result


def main():
    print("device:", torch.cuda.get_device_name(0))
    results = []
    for name, path in CHECKPOINTS.items():
        print(f"\n--- {name} ---", flush=True)
        r = run(name, path)
        results.append(r)
        print(json.dumps(r, indent=2, ensure_ascii=False))
    (HERE / "results.json").write_text(json.dumps(results, indent=2, ensure_ascii=False))
    print("\nwrote results.json")


if __name__ == "__main__":
    sys.exit(main())
