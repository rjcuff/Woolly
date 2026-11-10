# woolly-ai

Python client for [Woolly](https://usewoolly.com), the local memory for your AI tools.

Woolly keeps memories about the user on their own computer, keeps facts current, and shares them with every AI tool. This package lets your Python app use that memory.

## Install

```sh
pip install woolly-ai
```

You also need Woolly running on the same computer:

```sh
curl -fsSL https://usewoolly.com/install.sh | sh
woolly serve
```

## Use it

```python
import woolly

woolly.remember("Moved to Denver in October", key="user.location")
woolly.remember("I prefer short answers")

for m in woolly.recall("where do I live?"):
    print(m.content)
```

## Add memory to any AI app

`woolly.context()` returns a short note of relevant memories, ready to add to a prompt:

```python
import woolly
from openai import OpenAI

client = OpenAI()
question = "Any good hikes near me this weekend?"

response = client.chat.completions.create(
    model="gpt-4.1-mini",
    messages=[
        {"role": "system", "content": "You are a helpful assistant.\n\n" + woolly.context(question)},
        {"role": "user", "content": question},
    ],
)
```

It works the same with Anthropic, Ollama or any other model.

## More

```python
from woolly import Woolly

w = Woolly()                      # finds the server and token automatically
r = w.remember("Prefers spaces", key="code.indent", source="agent")
if r.saved_as_conflict:           # it disagrees with something the user said
    print("Needs review:", r.conflicts_with.content)

w.profile().markdown              # a summary of everything known
w.forget(memory_id)               # moves it to the trash
w.restore(memory_id)              # brings it back
```

Woolly refuses passwords, API keys and instruction-like text that doesn't come from the user. Those calls raise `woolly.WoollyError`.

Docs: [usewoolly.com/docs/python](https://usewoolly.com/docs/python)
