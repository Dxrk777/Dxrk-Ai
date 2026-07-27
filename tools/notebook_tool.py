# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
Notebook Tool Module — Jupyter notebook (.ipynb) cell editing.

Provides:
  - notebook_read        : dump notebook structure (cells, metadata)
  - notebook_edit_cell   : replace / insert / delete cells
  - notebook_execute_cell: mark a cell for execution (structural,
                           no kernel attached)

All operations use raw JSON (nbformat spec).  Notebook schema:
  {
    "nbformat": 4,
    "nbformat_minor": 5,
    "metadata": {...},
    "cells": [
      {
        "cell_type": "code" | "markdown",
        "id": "...",
        "source": "...",
        "metadata": {},
        "execution_count": null,
        "outputs": []
      }
    ]
  }
"""

import json
import logging
import os
import re
import uuid
from pathlib import Path

from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

_CELL_ID_RE = re.compile(r"cell-(\d+)")

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _validate_ipynb(path: str) -> tuple[str | None, dict | None]:
    """Validate *path* is a readable .ipynb file.  Returns (error, data)."""
    p = Path(path).resolve()
    if p.suffix != ".ipynb":
        return f"File must be a .ipynb notebook: {path}", None
    if not p.exists():
        return f"Notebook not found: {path}", None
    try:
        raw = p.read_text(encoding="utf-8")
    except Exception as e:
        return f"Cannot read notebook: {e}", None

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as e:
        return f"Notebook is not valid JSON: {e}", None

    if "cells" not in data or not isinstance(data["cells"], list):
        return "Notebook has no 'cells' array", None

    return None, data


def _resolve_cell_index(cells: list[dict], cell_id: str | None) -> int | None:
    """Resolve *cell_id* to an index.  cell_id can be a cell.id or 'cell-N'."""
    if cell_id is None:
        return 0

    # Try direct id match
    for i, cell in enumerate(cells):
        if cell.get("id") == cell_id:
            return i

    # Try cell-N numeric index
    m = _CELL_ID_RE.match(cell_id)
    if m:
        idx = int(m.group(1))
        if 0 <= idx < len(cells):
            return idx
        return None

    return None


def _make_cell(cell_type: str, source: str) -> dict:
    """Build a notebook cell dict."""
    cell_id = uuid.uuid4().hex[:13]
    base = {
        "cell_type": cell_type,
        "id": cell_id,
        "source": source,
        "metadata": {},
    }
    if cell_type == "code":
        base["execution_count"] = None
        base["outputs"] = []
    return base


def _language_from_metadata(metadata: dict) -> str:
    try:
        return metadata.get("language_info", {}).get("name", "python")
    except AttributeError:
        return "python"


# ---------------------------------------------------------------------------
# notebook_read handler
# ---------------------------------------------------------------------------

SCHEMA_READ = {
    "name": "notebook_read",
    "description": "Read a Jupyter notebook (.ipynb) and return its cells and metadata.",
    "parameters": {
        "type": "object",
        "properties": {
            "notebook_path": {
                "type": "string",
                "description": "Absolute path to the .ipynb file",
            },
        },
        "required": ["notebook_path"],
    },
}


def _handle_read(args: dict, **kw) -> str:
    path = args["notebook_path"]
    error, data = _validate_ipynb(path)
    if error:
        return tool_error(error)

    cells = []
    for cell in data["cells"]:
        cells.append({
            "id": cell.get("id"),
            "cell_type": cell.get("cell_type", "code"),
            "source": cell.get("source", ""),
            "execution_count": cell.get("execution_count"),
            "output_count": len(cell.get("outputs", [])),
        })

    return tool_result(
        notebook_path=str(Path(path).resolve()),
        nbformat=data.get("nbformat", 4),
        nbformat_minor=data.get("nbformat_minor", 0),
        language=_language_from_metadata(data.get("metadata", {})),
        cell_count=len(cells),
        cells=cells,
    )


# ---------------------------------------------------------------------------
# notebook_edit_cell handler
# ---------------------------------------------------------------------------

SCHEMA_EDIT = {
    "name": "notebook_edit_cell",
    "description": "Edit a cell in a Jupyter notebook. Supports replace (edit existing cell), insert (add new cell after a given cell), and delete (remove a cell).",
    "parameters": {
        "type": "object",
        "properties": {
            "notebook_path": {
                "type": "string",
                "description": "Absolute path to the .ipynb file",
            },
            "cell_id": {
                "type": "string",
                "description": "ID of the cell to edit. For insert, new cell goes after this cell (or at beginning if omitted). Accepts cell.id or 'cell-N' index.",
            },
            "new_source": {
                "type": "string",
                "description": "New source content for the cell",
            },
            "cell_type": {
                "type": "string",
                "enum": ["code", "markdown"],
                "description": "Cell type. Required when edit_mode=insert. Optional otherwise.",
            },
            "edit_mode": {
                "type": "string",
                "enum": ["replace", "insert", "delete"],
                "description": "Edit operation. Default: replace.",
            },
        },
        "required": ["notebook_path", "new_source"],
    },
}


def _handle_edit(args: dict, **kw) -> str:
    path = args["notebook_path"]
    new_source = args["new_source"]
    cell_id = args.get("cell_id")
    cell_type = args.get("cell_type")
    edit_mode = args.get("edit_mode", "replace")

    abs_path = str(Path(path).resolve())

    # -- validate file extension --
    if not abs_path.endswith(".ipynb"):
        return tool_error(f"File must be a .ipynb notebook: {path}")

    # -- validate edit_mode --
    if edit_mode not in ("replace", "insert", "delete"):
        return tool_error("edit_mode must be replace, insert, or delete")

    if edit_mode == "insert" and not cell_type:
        return tool_error("cell_type is required when edit_mode=insert")

    # -- read + parse --
    error, notebook = _validate_ipynb(abs_path)
    if error:
        return tool_error(error)

    cells = notebook["cells"]
    language = _language_from_metadata(notebook.get("metadata", {}))
    resolved_cell_id = cell_id

    # -- resolve index --
    cell_idx = _resolve_cell_index(cells, cell_id)

    if edit_mode != "insert" and cell_idx is None:
        return tool_error(f"Cell '{cell_id}' not found in notebook")

    if edit_mode == "insert":
        if cell_idx is not None:
            target_idx = cell_idx + 1
        else:
            target_idx = 0

        ct = cell_type or "code"
        new_cell = _make_cell(ct, new_source)
        cells.insert(target_idx, new_cell)
        resolved_cell_id = new_cell["id"]

    elif edit_mode == "delete":
        cells.pop(cell_idx)
        resolved_cell_id = cell_id

    else:
        ct = cell_type or cells[cell_idx].get("cell_type", "code")
        cells[cell_idx]["cell_type"] = ct
        cells[cell_idx]["source"] = new_source
        if ct == "code":
            cells[cell_idx]["execution_count"] = None
            cells[cell_idx]["outputs"] = []
        resolved_cell_id = cells[cell_idx].get("id", cell_id)

    # -- write back --
    try:
        content = json.dumps(notebook, indent=1, ensure_ascii=False)
        Path(abs_path).write_text(content, encoding="utf-8")
    except Exception as e:
        return tool_error(f"Failed to write notebook: {e}")

    return tool_result(
        notebook_path=abs_path,
        edit_mode=edit_mode,
        cell_id=resolved_cell_id,
        cell_type=cell_type or cells[min(cell_idx or 0, len(cells) - 1)].get("cell_type", "code") if cells else "code",
        language=language,
        new_source=new_source,
        cell_count=len(cells),
    )


# ---------------------------------------------------------------------------
# notebook_execute_cell handler
# ---------------------------------------------------------------------------

SCHEMA_EXECUTE = {
    "name": "notebook_execute_cell",
    "description": "Mark a cell for execution. This is a structural operation — it clears previous outputs and sets execution_count to None. Actual execution requires a Jupyter kernel (not provided by this tool).",
    "parameters": {
        "type": "object",
        "properties": {
            "notebook_path": {
                "type": "string",
                "description": "Absolute path to the .ipynb file",
            },
            "cell_id": {
                "type": "string",
                "description": "ID of the cell to execute. Accepts cell.id or 'cell-N' index.",
            },
        },
        "required": ["notebook_path", "cell_id"],
    },
}


def _handle_execute(args: dict, **kw) -> str:
    path = args["notebook_path"]
    cell_id = args["cell_id"]

    abs_path = str(Path(path).resolve())
    if not abs_path.endswith(".ipynb"):
        return tool_error(f"File must be a .ipynb notebook: {path}")

    error, notebook = _validate_ipynb(abs_path)
    if error:
        return tool_error(error)

    cell_idx = _resolve_cell_index(notebook["cells"], cell_id)
    if cell_idx is None:
        return tool_error(f"Cell '{cell_id}' not found in notebook")

    cell = notebook["cells"][cell_idx]
    if cell.get("cell_type") != "code":
        return tool_error("Only code cells can be executed")

    cell["execution_count"] = None
    cell["outputs"] = []

    try:
        content = json.dumps(notebook, indent=1, ensure_ascii=False)
        Path(abs_path).write_text(content, encoding="utf-8")
    except Exception as e:
        return tool_error(f"Failed to write notebook: {e}")

    source = cell.get("source", "")
    return tool_result(
        notebook_path=abs_path,
        cell_id=cell.get("id", cell_id),
        cell_type="code",
        language=_language_from_metadata(notebook.get("metadata", {})),
        source=source[:500] + ("..." if len(source) > 500 else ""),
        note="Cell marked for execution (no kernel attached — use Jupyter to actually run it)",
    )


# ---------------------------------------------------------------------------
# Registration
# ---------------------------------------------------------------------------

registry.register(
    name="notebook_read",
    toolset="dev",
    schema=SCHEMA_READ,
    handler=_handle_read,
    emoji="📓",
    max_result_size_chars=50_000,
)

registry.register(
    name="notebook_edit_cell",
    toolset="dev",
    schema=SCHEMA_EDIT,
    handler=_handle_edit,
    emoji="✏️",
    max_result_size_chars=10_000,
)

registry.register(
    name="notebook_execute_cell",
    toolset="dev",
    schema=SCHEMA_EXECUTE,
    handler=_handle_execute,
    emoji="▶️",
    max_result_size_chars=10_000,
)
