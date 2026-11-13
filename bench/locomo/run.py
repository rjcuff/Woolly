"""LOCOMO memory benchmark for Woolly, run entirely on your computer.

    python3 bench/locomo/run.py --conversations 1 --questions 30

It downloads the LOCOMO dataset (CC BY-NC 4.0, not included in this repo),
saves each conversation into Woolly, then for every question asks Woolly
for the closest memories, has a local model in Ollama answer from them, and
has a local model judge the answer against the correct one.

Needs a built woolly (make build) and Ollama with the answer model pulled.
Pull nomic-embed-text too for Woolly's search by meaning.
"""

import argparse
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "sdk" / "python"))
from woolly import Woolly  # noqa: E402

DATA_URL = "https://raw.githubusercontent.com/snap-research/locomo/main/data/locomo10.json"
CATEGORIES = {1: "multi-hop", 2: "temporal", 3: "open-domain", 4: "single-hop"}  # 5, adversarial, is left out
OLLAMA = os.environ.get("OLLAMA_HOST", "127.0.0.1:11434")
if "://" not in OLLAMA:
    OLLAMA = "http://" + OLLAMA

ANSWER_PROMPT = """You answer questions about conversations between {a} and {b}, using only the memories below.
Each memory starts with the date it was said. For questions about when something happened, work the date out from that date: if a memory from 8 May 2023 says "yesterday", the answer is 7 May 2023; "last week" means the week before; "next month" means the month after. Give dates as day month year.
Answer in a few words. If the memories don't contain the answer, say "I don't know".

Memories:
{memories}"""

JUDGE_PROMPT = """You grade answers to questions about a conversation, comparing them with a gold answer.
Be generous, the way published LOCOMO results grade: if the answer covers the same thing as the gold answer, mark it correct, even if it's worded differently, longer, or includes extra detail.
For questions about time, the gold answer may be a date, month or year, and the answer may be longer or use a different format. Mark it correct if it points to the same date or time period.
Mark it wrong if it says it doesn't know, misses the point of the gold answer, or contradicts it.
Reply with JSON only: {"correct": true} or {"correct": false}"""


def chat(model, system, user, fmt=None):
    body = {"model": model, "stream": False, "options": {"temperature": 0},
            "messages": [{"role": "system", "content": system}, {"role": "user", "content": user}]}
    if fmt:
        body["format"] = fmt
    req = urllib.request.Request(OLLAMA + "/api/chat", data=json.dumps(body).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=300) as r:
        return json.loads(r.read())["message"]["content"].strip()


def has_model(name):
    try:
        with urllib.request.urlopen(OLLAMA + "/api/tags", timeout=5) as r:
            names = {m["name"] for m in json.loads(r.read())["models"]}
    except OSError:
        return False
    return name in names or name + ":latest" in names


def load_data(cache):
    path = cache / "locomo10.json"
    if not path.exists():
        print("Downloading LOCOMO (CC BY-NC 4.0) from the snap-research/locomo repo...")
        cache.mkdir(parents=True, exist_ok=True)
        urllib.request.urlretrieve(DATA_URL, path)
    return json.loads(path.read_text())


def turns(conv):
    """Every message in a conversation as (dialog id, dated text), oldest first."""
    n = 1
    while f"session_{n}" in conv:
        date = conv.get(f"session_{n}_date_time", "")
        day = date.split(" on ")[-1] if " on " in date else date
        for t in conv[f"session_{n}"]:
            text = t.get("text", "").strip()
            if t.get("blip_caption"):
                text += f" [shares a photo: {t['blip_caption']}]"
            if text:
                yield t.get("dia_id", ""), f"[{day}] {t['speaker']}: {text}"
        n += 1


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--conversations", type=int, default=10, help="how many of the 10 conversations to run")
    ap.add_argument("--questions", type=int, default=0, help="questions per conversation (0 means all)")
    ap.add_argument("--model", default="qwen2.5:7b", help="Ollama model that answers")
    ap.add_argument("--judge", default="", help="Ollama model that grades (default: the answer model)")
    ap.add_argument("--top-k", type=int, default=20, help="memories recalled per question")
    ap.add_argument("--out", default=str(ROOT / "bench" / "locomo" / "results"), help="where to write results")
    args = ap.parse_args()
    judge = args.judge or args.model

    for m in {args.model, judge}:
        if not has_model(m):
            sys.exit(f"Ollama isn't running or {m} isn't pulled. Run: ollama pull {m}")
    semantic = has_model("nomic-embed-text")
    binary = os.environ.get("WOOLLY_BIN", str(ROOT / "bin" / "woolly"))
    if not Path(binary).exists():
        sys.exit("Build woolly first: make build")

    data = load_data(Path(args.out) / ".cache")[: args.conversations]
    home = tempfile.mkdtemp(prefix="woolly-locomo-")
    address = f"127.0.0.1:{free_port()}"
    env = dict(os.environ, WOOLLY_HOME=home)
    if not semantic:
        env["WOOLLY_NO_EMBED"] = "1"
    server = subprocess.Popen([binary, "serve", "--addr", address], env=env,
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    w = Woolly(address=address, home=home, app="LOCOMO benchmark", timeout=300)
    for _ in range(100):
        if w.is_running():
            break
        time.sleep(0.1)

    results, started = [], time.time()
    try:
        for conv_i, sample in enumerate(data):
            conv = sample["conversation"]
            project = f"locomo-{conv_i}"
            a, b = conv["speaker_a"], conv["speaker_b"]
            saved = 0
            by_id = {}
            for dia_id, text in turns(conv):
                by_id[dia_id] = text
                try:
                    w.remember(text, project=project, source="import")
                    saved += 1
                except Exception:
                    pass  # Woolly refuses a few, like text that looks like a secret
            qa = [q for q in sample["qa"] if q.get("category") in CATEGORIES and "answer" in q]
            if args.questions:
                qa = qa[: args.questions]
            print(f"\nConversation {conv_i + 1}/{len(data)}: {a} and {b}, {saved} messages saved, {len(qa)} questions")
            for i, q in enumerate(qa):
                t0 = time.time()
                found = w.recall(q["question"], limit=args.top_k, project=project)
                recall_ms = (time.time() - t0) * 1000
                memories = "\n".join("- " + m.content for m in found) or "(none)"
                # Did search find a message that holds the answer? This part is all Woolly.
                evidence = {by_id[e].strip() for e in q.get("evidence", []) if e in by_id}
                hit = bool(evidence & {m.content.strip() for m in found}) if evidence else None
                answer = chat(args.model, ANSWER_PROMPT.format(a=a, b=b, memories=memories), q["question"])
                verdict = chat(judge, JUDGE_PROMPT,
                               f"Question: {q['question']}\nGold answer: {q['answer']}\nAnswer to grade: {answer}",
                               {"type": "object", "properties": {"correct": {"type": "boolean"}}, "required": ["correct"]})
                try:
                    correct = bool(json.loads(verdict)["correct"])
                except (ValueError, KeyError):
                    correct = False
                results.append({"conversation": conv_i, "category": CATEGORIES[q["category"]], "question": q["question"],
                                "gold": str(q["answer"]), "answer": answer, "correct": correct, "found_evidence": hit,
                                "recall_ms": round(recall_ms, 1)})
                done = sum(r["correct"] for r in results)
                print(f"\r  {i + 1}/{len(qa)}  running score {done / len(results):.1%}", end="", flush=True)
            print()
    finally:
        server.terminate()
        server.wait()
        shutil.rmtree(home, ignore_errors=True)

    if not results:
        sys.exit("No questions were run.")
    summary = {"overall": sum(r["correct"] for r in results) / len(results), "questions": len(results)}
    for cat in CATEGORIES.values():
        rs = [r for r in results if r["category"] == cat]
        if rs:
            summary[cat] = {"score": sum(r["correct"] for r in rs) / len(rs), "questions": len(rs)}
    graded = [r for r in results if r["found_evidence"] is not None]
    if graded:
        summary["retrieval"] = sum(r["found_evidence"] for r in graded) / len(graded)
    recall_times = sorted(r["recall_ms"] for r in results)
    summary["recall_ms_median"] = recall_times[len(recall_times) // 2]
    summary["setup"] = {"answer_model": args.model, "judge_model": judge, "top_k": args.top_k,
                        "search": "keyword + meaning (nomic-embed-text)" if semantic else "keyword only",
                        "conversations": len(data), "minutes": round((time.time() - started) / 60, 1)}

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    stamp = time.strftime("%Y%m%d-%H%M%S")
    (out / f"{stamp}.json").write_text(json.dumps({"summary": summary, "results": results}, indent=2))

    print(f"\nLOCOMO with Woolly ({summary['setup']['search']}, {args.model}, top {args.top_k})")
    for cat in CATEGORIES.values():
        if cat in summary:
            s = summary[cat]
            print(f"  {cat:<12} {s['score']:6.1%}  ({s['questions']} questions)")
    print(f"  {'overall':<12} {summary['overall']:6.1%}  ({summary['questions']} questions)")
    if "retrieval" in summary:
        print(f"  Woolly found a message with the answer for {summary['retrieval']:.1%} of questions (top {args.top_k})")
    print(f"  median recall {summary['recall_ms_median']:.0f} ms. Full results: {out / (stamp + '.json')}")


if __name__ == "__main__":
    main()
