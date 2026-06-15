import argparse
import getpass
import json
import os
import shlex
import subprocess
import sys
import urllib.request


PUBLIC_HOST = "140.143.209.222"
AUTH_URL = f"http://{PUBLIC_HOST}:7500/api/user/activate"
USERPROFILE = os.environ["USERPROFILE"]
TOKEN_PATH = os.path.join(USERPROFILE, ".frp-ssh", "token.xml")
PYDEPS = os.path.join(USERPROFILE, ".frp-ssh", "pydeps")
PUBLIC_KEY_PATH = os.path.join(USERPROFILE, ".ssh", "id_ed25519.pub")
PRIVATE_KEY_PATH = os.path.join(USERPROFILE, ".ssh", "id_ed25519")

SERVICES = {
    "114": {"port": 6222, "target_ip": "210.47.163.114", "user": "root"},
    "113": {"port": 6223, "target_ip": "210.47.163.113", "user": "root"},
    "118": {"port": 6224, "target_ip": "210.47.163.118", "user": "root"},
    "181": {"port": 6225, "target_ip": "210.47.163.181", "user": "root"},
    "103": {"port": 6226, "target_ip": "10.2.0.3", "user": "root"},
}

sys.path.insert(0, PYDEPS)
import paramiko  # noqa: E402


def read_token():
    ps = (
        "$s=Import-Clixml -LiteralPath "
        + repr(TOKEN_PATH)
        + ";"
        + "$b=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($s);"
        + "try{[Runtime.InteropServices.Marshal]::PtrToStringBSTR($b)}"
        + "finally{[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b)}"
    )
    result = subprocess.run(
        ["powershell", "-NoProfile", "-Command", ps],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def activate(token, port):
    data = json.dumps({"token": token, "port": port}).encode()
    req = urllib.request.Request(
        AUTH_URL,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=15) as resp:
        resp.read()


def install_key(service, password, public_key):
    port = service["port"]
    user = service["user"]
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        PUBLIC_HOST,
        port=port,
        username=user,
        password=password,
        look_for_keys=False,
        allow_agent=False,
        timeout=15,
        auth_timeout=15,
        banner_timeout=15,
    )
    quoted_key = shlex.quote(public_key)
    command = (
        "mkdir -p ~/.ssh && chmod 700 ~/.ssh && "
        "touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && "
        f"grep -qxF {quoted_key} ~/.ssh/authorized_keys || "
        f"printf '%s\\n' {quoted_key} >> ~/.ssh/authorized_keys"
    )
    stdin, stdout, stderr = client.exec_command(command, timeout=15)
    code = stdout.channel.recv_exit_status()
    err = stderr.read().decode(errors="replace").strip()
    client.close()
    if code != 0:
        raise RuntimeError(err or f"remote command failed with exit code {code}")


def test_key(alias):
    result = subprocess.run(
        [
            "ssh",
            "-o",
            "BatchMode=yes",
            "-o",
            "ConnectTimeout=8",
            "-i",
            PRIVATE_KEY_PATH,
            alias,
            "hostname; whoami",
        ],
        capture_output=True,
        text=True,
        timeout=15,
    )
    return result.returncode, result.stdout.strip(), result.stderr.strip()


def main():
    parser = argparse.ArgumentParser(description="Install local SSH public key on FRP-managed SSH targets.")
    parser.add_argument("targets", nargs="*", default=list(SERVICES.keys()), help="Targets: 114 113 118 181 103")
    parser.add_argument("--same-password", action="store_true", help="Prompt once and reuse the password for all targets.")
    parser.add_argument("--password-env", default="", help="Read password from this environment variable.")
    args = parser.parse_args()

    targets = []
    for target in args.targets:
        if target not in SERVICES:
            raise SystemExit(f"unknown target {target}; available: {', '.join(SERVICES)}")
        targets.append(target)

    with open(PUBLIC_KEY_PATH, "r", encoding="utf-8") as f:
        public_key = f.read().strip()
    token = read_token()

    shared_password = None
    if args.password_env:
        shared_password = os.environ.get(args.password_env)
        if not shared_password:
            raise SystemExit(f"environment variable {args.password_env} is empty")
    elif args.same_password:
        shared_password = getpass.getpass("root password for all selected targets: ")

    for target in targets:
        service = SERVICES[target]
        label = f"{target} ({service['target_ip']} via {PUBLIC_HOST}:{service['port']})"
        print(f"==> {label}")
        activate(token, service["port"])
        password = shared_password or getpass.getpass(f"{service['user']} password for {target}: ")
        install_key(service, password, public_key)
        code, out, err = test_key(target)
        if code == 0:
            print(f"OK {target}: {out}")
        else:
            print(f"WARN {target}: key installed but test failed: {err}", file=sys.stderr)


if __name__ == "__main__":
    main()
