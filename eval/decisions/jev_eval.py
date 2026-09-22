"""The same labelled sets through Jev on OpenRouter, scored identically to the local Laya run."""

import json, os, statistics, time
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import subprocess

from evaluate import DATA, REASON_Q, SEARCH_Q, YESNO_Q, ece

HERE = Path(__file__).parent
MODEL = os.environ.get("JEV_MODEL", "typesafe/jev-1.13")
KEY = Path(os.path.expanduser("~/dev/casap/.env.openrouter-api-key")).read_text().strip()
COST = []


def decide(state, questions, attempts=4):
    """Shelled out to curl: python's own HTTP stack cannot reach the network on this machine."""
    payload = json.dumps({"model": MODEL, "state": state, "questions": questions})
    cmd = ["curl", "-s", "--max-time", "60", "-X", "POST",
           "https://openrouter.ai/api/alpha/decisions",
           "-H", f"Authorization: Bearer {KEY}", "-H", "Content-Type: application/json",
           "--data-binary", "@-"]
    for attempt in range(attempts):
        out = subprocess.run(cmd, input=payload, capture_output=True, text=True, timeout=90).stdout
        try:
            body = json.loads(out)
            COST.append(body.get("usage", {}).get("cost", 0))
            return body["answers"]
        except Exception:
            if attempt == attempts - 1:
                raise RuntimeError(f"decisions API said: {out[:200]}")
            time.sleep(1.5 * (attempt + 1))
    raise RuntimeError("unreachable")


def main():
    result = {"checkpoint": f"jev ({MODEL})"}

    single = {"body": "I did not make this payment."}
    decide(single, REASON_Q)
    lat = sorted((lambda: ((lambda t: (decide(single, REASON_Q), (time.perf_counter() - t) * 1000)[1])(time.perf_counter())))() for _ in range(10))
    result["latency_ms"] = {"one_question_median": round(statistics.median(lat), 1), "one_question_p90": round(lat[8], 1)}

    with ThreadPoolExecutor(max_workers=8) as pool:
        # ---- reason
        cases = json.loads((DATA / "reasons.json").read_text())
        answers = list(pool.map(lambda c: decide({"body": c["text"]}, REASON_Q), cases))
        hits, by_band, tb, by_lang, tl, confusion, calib = 0, Counter(), Counter(), Counter(), Counter(), Counter(), []
        for case, answer in zip(cases, answers):
            got = answer["reason"]["choice"]
            ok = got == case["reason"]
            hits += ok
            by_band[case["band"]] += ok; tb[case["band"]] += 1
            by_lang[case["lang"]] += ok; tl[case["lang"]] += 1
            confusion[(case["reason"], got)] += 1
            calib.append((float(answer["reason"].get("confidence") or 0), ok))
        result["reason"] = {"accuracy": round(hits / len(cases), 3), "n": len(cases),
                            "by_band": {b: f"{by_band[b]}/{tb[b]}" for b in tb},
                            "by_lang": {l: f"{by_lang[l]}/{tl[l]}" for l in tl},
                            "ece": round(ece(calib), 3),
                            "confusion": {f"{w}->{g}": n for (w, g), n in sorted(confusion.items()) if w != g}}

        # ---- questionnaire
        cases = json.loads((DATA / "questionnaire.json").read_text())
        answers = list(pool.map(lambda c: decide({"reply": c["reply"]}, YESNO_Q), cases))
        hits = total = 0
        per_q, tq, by_band, tb, calib = Counter(), Counter(), Counter(), Counter(), []
        for case, answer in zip(cases, answers):
            for key, want in case["want"].items():
                p = float(answer[key]["noul"])
                got = "yes" if p >= 0.5 else "no"
                ok = got == want
                hits += ok; total += 1
                per_q[key] += ok; tq[key] += 1
                by_band[case["band"]] += ok; tb[case["band"]] += 1
                calib.append((p if got == "yes" else 1 - p, ok))
        result["questionnaire"] = {"accuracy": round(hits / total, 3), "n": total,
                                   "per_question": {q: f"{per_q[q]}/{tq[q]}" for q in tq},
                                   "by_band": {b: f"{by_band[b]}/{tb[b]}" for b in tb},
                                   "ece": round(ece(calib), 3)}

        # ---- search
        cases = json.loads((DATA / "search.json").read_text())
        answers = list(pool.map(lambda c: decide({"query": c["query"]}, SEARCH_Q), cases))
        hits = total = exact = 0
        per_field, tf, by_band, tb = Counter(), Counter(), Counter(), Counter()
        for case, answer in zip(cases, answers):
            got = {"state": answer["state"]["choice"], "reason": answer["reason"]["choice"],
                   "overdue": "yes" if float(answer["overdue"]["noul"]) >= 0.5 else "no"}
            all_ok = True
            for field, want in case["want"].items():
                ok = got[field] == want
                hits += ok; total += 1; all_ok &= ok
                per_field[field] += ok; tf[field] += 1
                by_band[case["band"]] += ok; tb[case["band"]] += 1
            exact += all_ok
        result["search"] = {"field_accuracy": round(hits / total, 3), "n_fields": total,
                            "query_exact": f"{exact}/{len(cases)}",
                            "per_field": {f: f"{per_field[f]}/{tf[f]}" for f in tf},
                            "by_band": {b: f"{by_band[b]}/{tb[b]}" for b in tb}}

    result["calls"] = len(COST)
    result["cost_usd"] = round(sum(COST), 5)
    print(json.dumps(result, indent=2, ensure_ascii=False))
    (HERE / "results-jev.json").write_text(json.dumps([result], indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
