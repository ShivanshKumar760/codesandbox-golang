"""The runner: runs inside a fresh Cloud Run Job container, executes ONE user
program (Python, Node.js or Go), writes the result, and exits.

Environment variables:
  RUNS_BUCKET - the Cloud Storage bucket to use
  RUN_ID      - which run this container is responsible for
  LANGUAGE    - python, node or go
"""
import json
import os
import shutil
import signal
import subprocess
import sys
import tempfile

RUN_TIMEOUT = 5          # seconds the user's program may run
COMPILE_TIMEOUT = 15     # seconds the Go compiler may take
MAX_OUTPUT = 64 * 1024   # keep at most 64 KB of stdout and 64 KB of stderr
MAX_FILE_SIZE = 1024 * 1024  # the program may write at most 1 MiB to any file (including its output)

USER_PATH = "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"  # the only PATH user code sees
GO_CACHE = "/opt/gocache"  # Go's build cache, warmed up when the image is built

# What to do for each language: the file name, an optional compile command, the run command.
LANGUAGES = {
    "python": {"filename": "main.py", "compile": None, "run": ["python3", "-I", "-u", "main.py"]},
    "node": {"filename": "main.js", "compile": None, "run": ["node", "main.js"]},
    "go": {
        "filename": "main.go",
        "compile": ["go", "build", "-o", "prog", "main.go"],
        "run": ["./prog"],
    },
}


def cut_output(data):
    """Turn bytes into text and shorten very long output."""
    text = data[:MAX_OUTPUT].decode("utf-8", errors="replace")
    if len(data) > MAX_OUTPUT:
        text += "\n...output cut..."
    return text


def run_step(command, workdir, env, timeout, limit_file_size):
    """Run one command and report what happened (a dictionary)."""
    if limit_file_size:
        # prlimit starts the command with a cap on how big a file it may write
        command = ["prlimit", "--fsize=%d" % MAX_FILE_SIZE, "--"] + command

    # Output goes into anonymous temporary files, not pipes. A program that
    # leaves a background process behind cannot make us wait for a pipe to close.
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        process = subprocess.Popen(
            command,
            cwd=workdir,
            env=env,
            stdin=subprocess.DEVNULL,  # the program cannot read input
            stdout=out,
            stderr=err,
            start_new_session=True,  # its own process group, so we can stop all of it
        )
        timed_out = False
        try:
            exit_code = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
            exit_code = -1

        # Stop everything the program started (also leftover background processes).
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass  # nothing was left
        process.wait()  # collect the exit status so no zombie remains

        out.seek(0)
        err.seek(0)
        return {
            "stdout": cut_output(out.read(MAX_OUTPUT + 1)),
            "stderr": cut_output(err.read(MAX_OUTPUT + 1)),
            "exit_code": exit_code,  # 0 = ok, 1 or more = error, negative = killed by a signal
            "timed_out": timed_out,
        }


def run_code(language, code):
    """Save the code in a private folder, compile it if needed, run it."""
    spec = LANGUAGES[language]
    workdir = tempfile.mkdtemp(prefix="run-")  # a new private folder
    try:
        with open(os.path.join(workdir, spec["filename"]), "wb") as f:
            f.write(code)

        # The program only sees these variables. No secrets, no RUN_ID.
        env = {"PATH": USER_PATH, "HOME": workdir}

        if spec["compile"]:
            compile_env = dict(env)
            compile_env.update(
                GOCACHE=GO_CACHE,       # reuse the warmed-up cache
                GOPROXY="off",          # never download packages
                GOTOOLCHAIN="local",    # never download another Go version
                CGO_ENABLED="0",        # plain Go only, no C compiler needed
            )
            result = run_step(spec["compile"], workdir, compile_env, COMPILE_TIMEOUT, False)
            if result["timed_out"]:
                result["stderr"] += "\ncompile timed out after %d seconds" % COMPILE_TIMEOUT
            if result["exit_code"] != 0:
                return result  # the compiler's messages are the answer

        return run_step(spec["run"], workdir, env, RUN_TIMEOUT, True)
    finally:
        shutil.rmtree(workdir, ignore_errors=True)  # delete the folder


def main():
    run_id = os.environ.get("RUN_ID", "")
    bucket_name = os.environ.get("RUNS_BUCKET", "")
    language = os.environ.get("LANGUAGE", "")
    if not run_id or not bucket_name or language not in LANGUAGES:
        sys.exit("RUN_ID, RUNS_BUCKET and LANGUAGE (python, node or go) are required")

    from google.cloud import storage  # imported here so the code above can be tested without it

    bucket = storage.Client().bucket(bucket_name)

    # Step 1: download the code the API uploaded.
    code = bucket.blob("runs/%s/code" % run_id).download_as_bytes()

    # Step 2: run it.
    result = run_code(language, code)

    # Step 3: upload the result for the API to read.
    bucket.blob("runs/%s/result.json" % run_id).upload_from_string(
        json.dumps(result), content_type="application/json"
    )

    # We exit normally (code 0) even if the USER's program failed. That failure
    # is stored inside result.json. Any crash above ends with exit code 1 =
    # "the runner failed".
    print("done", run_id, language, "exit_code", result["exit_code"], "timed_out", result["timed_out"])


if __name__ == "__main__":
    main()
