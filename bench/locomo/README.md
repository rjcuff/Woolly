# LOCOMO benchmark

How well Woolly remembers long conversations, measured with [LOCOMO](https://github.com/snap-research/locomo), the benchmark mem0 and others report. Everything runs on your computer: Woolly for memory, Ollama for answering and grading. No API keys.

## Run it

```sh
make build
ollama pull qwen2.5:7b
ollama pull nomic-embed-text
python3 bench/locomo/run.py --conversations 1 --questions 30   # a quick try, about 3 minutes
python3 bench/locomo/run.py                                      # all 10 conversations
```

Options: `--model` (answer model, default `qwen2.5:7b`), `--judge` (grading model, default the answer model), `--top-k` (memories per question, default 20), `--conversations`, `--questions`.

Results go to `bench/locomo/results/`, which isn't committed.

## How it works

1. Downloads `locomo10.json`. The dataset is CC BY-NC 4.0, so it isn't included in this repo.
2. Starts its own Woolly with an empty, temporary memory folder.
3. Saves every message of each conversation as a memory, starting with the date it was said, one project per conversation.
4. For each question, asks Woolly for the 20 closest memories, has the answer model answer from only those, and has the judge compare the answer with the correct one.
5. Reports accuracy by category, how often Woolly's results included a message that holds the answer, and how long recall took.

Adversarial questions (category 5) are left out, as in mem0's paper. The judge is generous in the same way published LOCOMO results are: an answer that says the same thing in other words counts.

## Results

A first sample: the first 30 questions of conversation 1, with `qwen2.5:7b` answering and grading and `nomic-embed-text` for search, on a MacBook.

| | Score |
|---|---|
| Multi-hop | 70.0% (10 questions) |
| Temporal | 56.2% (16 questions) |
| Open-domain | 25.0% (4 questions) |
| **Overall** | **56.7%** (30 questions) |
| Woolly found the message with the answer | 86.7% |
| Median recall time | 23 ms |

A run over all 10 conversations is next. With a 7B model it takes a few hours.

## Reading the numbers

Two things are measured here, and they're worth keeping apart:

- **Retrieval** is Woolly's job: did the memories it returned include the message with the answer?
- **Accuracy** also depends on the answer model. A 7B model on a laptop gets dates and multi-step questions wrong more often than GPT-4o, which is what most published numbers use.

So these numbers aren't directly comparable to mem0's, which use hosted models for both answering and grading. Pass `--model` and `--judge` to try other models.
