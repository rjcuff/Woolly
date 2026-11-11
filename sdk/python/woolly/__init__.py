"""Python client for Woolly, the local memory for your AI tools.

Woolly runs on your own computer. Start it with ``woolly serve``, then:

    import woolly

    woolly.remember("I prefer short answers")
    print(woolly.recall("how should replies be written?"))

    # Add what Woolly knows to any AI prompt
    system_prompt = "You are a helpful assistant.\\n\\n" + woolly.context(user_message)

Everything stays on your computer. See https://usewoolly.com/docs/python.
"""

from .client import (
    Memory,
    Profile,
    RememberResult,
    Woolly,
    WoollyError,
    WoollyNotRunning,
)

__all__ = [
    "Memory",
    "Profile",
    "RememberResult",
    "Woolly",
    "WoollyError",
    "WoollyNotRunning",
    "context",
    "forget",
    "learn",
    "profile",
    "recall",
    "remember",
]

__version__ = "0.3.0"

_default = None


def _client() -> Woolly:
    global _default
    if _default is None:
        _default = Woolly()
    return _default


def remember(content, key=None, tags=None, source="user"):
    """Save a memory. See :meth:`Woolly.remember`."""
    return _client().remember(content, key=key, tags=tags, source=source)


def recall(query, limit=5):
    """Find memories relevant to ``query``. See :meth:`Woolly.recall`."""
    return _client().recall(query, limit=limit)


def forget(memory_id):
    """Move a memory to the trash. See :meth:`Woolly.forget`."""
    return _client().forget(memory_id)


def profile():
    """Get a summary of what Woolly knows. See :meth:`Woolly.profile`."""
    return _client().profile()


def context(query, limit=5):
    """Get a note of relevant memories to add to a prompt. See :meth:`Woolly.context`."""
    return _client().context(query, limit=limit)


def learn(text, project=None, dry_run=False):
    """Pick out facts from a conversation with a local model and save them."""
    return _client().learn(text, project=project, dry_run=dry_run)
