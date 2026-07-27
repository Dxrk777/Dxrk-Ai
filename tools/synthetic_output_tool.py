# SPDX-License-Identifier: MIT
import json

from tools.registry import registry, tool_error, tool_result

SCHEMA = {
    "name": "produce_output",
    "description": "Produce structured output in the requested format (json, text, or markdown). Use this to present formatted results to the user with optional title.",
    "parameters": {
        "type": "object",
        "properties": {
            "data": {
                "type": "object",
                "description": "Structured output data to present to the user.",
            },
            "format": {
                "type": "string",
                "enum": ["json", "text", "markdown"],
                "description": "Output format: json, text, or markdown.",
                "default": "text",
            },
            "title": {
                "type": "string",
                "description": "Optional title for the output.",
            },
        },
        "required": ["data"],
    },
}


def _handler(args, **kw):
    data = args.get("data")
    if data is None:
        return tool_error("'data' is required")

    fmt = args.get("format", "text") or "text"
    title = args.get("title")

    if fmt == "json":
        body = json.dumps(data, indent=2, ensure_ascii=False)
    elif fmt == "markdown":
        if isinstance(data, dict):
            parts = []
            for k, v in data.items():
                if isinstance(v, (dict, list)):
                    parts.append(
                        f"**{k}:**\n```\n{json.dumps(v, indent=2, ensure_ascii=False)}\n```"
                    )
                else:
                    parts.append(f"**{k}:** {v}")
            body = "\n\n".join(parts)
        else:
            body = str(data)
    else:
        if isinstance(data, dict):
            body = "\n".join(
                f"{k}: {json.dumps(v, ensure_ascii=False) if isinstance(v, (dict, list)) else v}"
                for k, v in data.items()
            )
        else:
            body = str(data)

    output = body
    if title:
        output = f"{title}\n{'=' * len(title)}\n{body}"

    return tool_result(output=output, format=fmt, title=title)


registry.register(
    name="produce_output",
    toolset="system",
    schema=SCHEMA,
    handler=_handler,
    emoji="\U0001f4e4",
)
