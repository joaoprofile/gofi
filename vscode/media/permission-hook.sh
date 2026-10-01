#!/bin/sh
# PreToolUse hook: asks the GOFI AI panel before the engine runs a tool.
#
# The engine runs this as a separate process, feeding the tool call on stdin.
# Exiting 0 lets the call through; exiting 2 blocks it and hands stderr back to
# the model as the reason. Everything in between is a handshake over files,
# because that is the only channel a hook process and the extension host both
# have without either of them opening a port.
#
# Written in POSIX sh rather than Node so it does not depend on a `node` on
# PATH — the engine's own runtime is bundled and not necessarily exposed.
#
#   $1  directory the extension watches
#
# It fails closed: any confusion — no answer, no directory, a panel that went
# away — denies. A gate whose failure mode is "allow" is not a gate.

DIR="$1"
TIMEOUT_TENTHS=3000 # 5 minutes

if [ -z "$DIR" ] || [ ! -d "$DIR" ]; then
	echo "GOFI AI: painel de aprovação indisponível, alteração bloqueada." >&2
	exit 2
fi

ID="$$-$(awk 'BEGIN{srand();printf "%d", rand()*1000000}')"
REQUEST="$DIR/$ID.request"
ALLOW="$DIR/$ID.allow"
DENY="$DIR/$ID.deny"

# Write to a temp name first, then move: the watcher must never see half a
# request and try to parse it.
cat > "$REQUEST.partial"
mv "$REQUEST.partial" "$REQUEST"

i=0
while [ "$i" -lt "$TIMEOUT_TENTHS" ]; do
	if [ -f "$ALLOW" ]; then
		rm -f "$ALLOW" "$REQUEST"
		exit 0
	fi
	if [ -f "$DENY" ]; then
		REASON=$(cat "$DENY" 2>/dev/null)
		rm -f "$DENY" "$REQUEST"
		[ -z "$REASON" ] && REASON="O usuário não autorizou esta alteração."
		echo "$REASON" >&2
		exit 2
	fi
	sleep 0.1
	i=$((i + 1))
done

rm -f "$REQUEST"
echo "GOFI AI: ninguém respondeu ao pedido de aprovação a tempo; alteração bloqueada." >&2
exit 2
