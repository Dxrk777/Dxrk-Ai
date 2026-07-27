# SPDX-License-Identifier: MIT
import json
import os
import time
import uuid
from pathlib import Path

from tools.registry import registry, tool_error

QUESTIONS_DIR = Path.home() / ".dxrk" / "questions"

SCHEMA = {
    "name": "ask_user",
    "description": (
        "Ask the user a question and wait for their response. "
        "Use when you need clarification, a decision, or input from the user "
        "to proceed with a task. Supports optional predefined options. "
        "In blocking mode the tool waits until the user responds; "
        "in non-blocking mode it returns immediately and the answer "
        "can be retrieved later."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "question": {
                "type": "string",
                "description": "The question to ask the user."
            },
            "options": {
                "type": "array",
                "items": {"type": "string"},
                "description": (
                    "Optional predefined answer choices. "
                    "If provided, the user selects one. "
                    "If omitted, the user types a free-form answer."
                ),
            },
            "blocking": {
                "type": "boolean",
                "description": (
                    "If true (default), wait for the user to answer. "
                    "If false, return immediately with pending=true "
                    "and the answer can be picked up later."
                ),
                "default": True,
            },
        },
        "required": ["question"],
    },
}


def _ensure_questions_dir():
    QUESTIONS_DIR.mkdir(parents=True, exist_ok=True)


def _wait_for_answer(question_file: Path, poll_interval: float = 0.5, timeout: float = 86400.0):
    """Poll for the response file. Returns the parsed answer dict or None on timeout."""
    start = time.time()
    while time.time() - start < timeout:
        if not question_file.exists():
            time.sleep(poll_interval)
            continue
        try:
            data = json.loads(question_file.read_text(encoding="utf-8"))
            return data
        except (json.JSONDecodeError, OSError):
            time.sleep(poll_interval)
            continue
    return None


async def _handler(args, **kw):
    question = args.get("question", "").strip()
    if not question:
        return tool_error("'question' is required and must be non-empty")

    options = args.get("options")
    blocking = args.get("blocking", True)

    _ensure_questions_dir()

    qid = str(uuid.uuid4())
    question_file = QUESTIONS_DIR / f"{qid}.json"
    answer_file = QUESTIONS_DIR / f"{qid}.answer.json"

    payload = {
        "id": qid,
        "question": question,
        "options": options,
        "blocking": blocking,
        "created_at": time.time(),
    }
    question_file.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")

    if not blocking:
        return json.dumps({
            "id": qid,
            "pending": True,
            "question": question,
            "note": "Question sent to user. The answer file will appear at ~/.dxrk/questions/{qid}.answer.json when answered.",
        })

    answer_data = _wait_for_answer(answer_file)
    if answer_data is None:
        return json.dumps({
            "id": qid,
            "error": "Timed out waiting for user response",
            "question": question,
        })

    return json.dumps({
        "id": qid,
        "answer": answer_data.get("answer", ""),
        "selected": answer_data.get("selected"),
        "question": question,
    })


registry.register(
    name="ask_user",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    is_async=True,
    emoji="💬",
)
