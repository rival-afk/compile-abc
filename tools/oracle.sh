#!/data/data/com.termux/files/usr/bin/bash
# tools/oracle.sh - compile and run a .pas file with the ORIGINAL PascalABC.NET
# compiler (running under proot ubuntu + .NET 10).  Used as a golden reference
# for the abc compiler's tests.
#
# Usage: tools/oracle.sh <file.pas> [args...]        # compile + run
#        tools/oracle.sh -c <file.pas>               # compile only
#
# Notes:
#  * `script` allocates a pseudo terminal: pabcnetc calls Console.ReadKey after
#    a compile error, which throws when stdin is redirected.  The trailing
#    newline dismisses that prompt, and `-e` propagates the compiler's status.
#  * `proot-distro login` drops custom environment variables, so the command
#    is written to a script file (visible inside the container) instead of
#    being passed through the environment.
set -euo pipefail

ROOTFS=/data/data/com.termux/files/usr/var/lib/proot-distro/containers/ubuntu/rootfs
PABC=/opt/pabc
TMP=${TMPDIR:-/data/data/com.termux/files/usr/tmp}

compile_only=0
if [ "${1:-}" = "-c" ]; then
  compile_only=1
  shift
fi

src=$(realpath "$1")
shift || true
exepath="${src%.pas}.exe"

# remove a stale executable so a failed compile cannot run an old binary
rm -f "$exepath"

cmdfile="$TMP/pabc_oracle_$$.sh"
trap 'rm -f "$cmdfile"' EXIT

{
  printf 'set -e\n'
  printf 'cd %q\n' "$PABC"
  printf 'exec dotnet pabcnetc.dll %q\n' "$src"
} > "$cmdfile"

if ! printf '\n' | script -qec \
    "proot-distro login ubuntu -- bash $(printf '%q' "$cmdfile")" /dev/null; then
  exit 1
fi

if [ "$compile_only" -eq 0 ]; then
  {
    printf 'set -e\n'
    printf 'cd %q\n' "$(dirname "$exepath")"
    printf 'exec dotnet %q' "$(basename "$exepath")"
    for a in "$@"; do printf ' %q' "$a"; done
    printf '\n'
  } > "$cmdfile"
  proot-distro login ubuntu -- bash "$cmdfile"
fi
