"""Terminal-Bench agent that runs pi, so it can be measured on the same row.

pi is installed into the task container and run there, which is the pattern
Terminal-Bench's own npm-based agents use. Nothing here is bluecollar's: it
exists so the comparison holds the benchmark, the model and the task fixed and
changes only the harness.
"""

import os
import shlex

from terminal_bench.agents.installed_agents.abstract_installed_agent import (
    AbstractInstalledAgent,
)
from terminal_bench.terminal.models import TerminalCommand

PASSED_THROUGH_KEYS = ("PI_BASE_URL", "PI_MODEL_ID")


class PiAgent(AbstractInstalledAgent):
    @staticmethod
    def name() -> str:
        return "pi"

    def __init__(self, model_name: str, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._model_name = model_name
        self._version = kwargs.get("version", "latest")

    @property
    def _env(self) -> dict[str, str]:
        return {key: os.environ[key] for key in PASSED_THROUGH_KEYS if key in os.environ}

    @property
    def _install_agent_script_path(self) -> os.PathLike:
        return self._get_templated_script_path("pi-setup.sh.j2")

    def _run_agent_commands(self, instruction: str) -> list[TerminalCommand]:
        return [
            TerminalCommand(
                command=(
                    f"pi --provider local --model {shlex.quote(self._model_name)} "
                    f"--api-key local -p {shlex.quote(instruction)}"
                ),
                min_timeout_sec=0.0,
                max_timeout_sec=float("inf"),
                block=True,
                append_enter=True,
            ),
        ]
