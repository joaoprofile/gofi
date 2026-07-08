#!/usr/bin/env bash
#
# gen-index.sh — regenera o manifesto de retrieval (RAG) de um corpus a partir
# do frontmatter dos seus documentos. NÃO edite o INDEX.md à mão: ele é derivado.
#
# Uso:
#   bash .claude/scripts/gen-index.sh specs   # regenera specs/INDEX.md
#   bash .claude/scripts/gen-index.sh prd     # regenera prd/INDEX.md
#
# Lê de cada <corpus>/**/*.md (exceto o próprio INDEX.md) os campos de
# frontmatter: contexto, submodulo, versao, status, keywords. Monta a tabela
# ordenada por contexto e submódulo. Roda a partir da raiz do projeto.
set -euo pipefail

corpus="${1:-}"
case "$corpus" in
  specs) title="Índice de Specs (SDD)" ;;
  prd)   title="Índice de PRDs" ;;
  *)
    echo "uso: gen-index.sh {specs|prd}" >&2
    exit 2
    ;;
esac

if [ ! -d "$corpus" ]; then
  echo "diretório '$corpus/' não existe — nada a indexar" >&2
  exit 1
fi

index="$corpus/INDEX.md"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

# Cabeçalho canônico (idêntico ao seed da CLI).
{
  echo "# $title — manifesto de retrieval (RAG)"
  echo ""
  echo "> **Derivado do frontmatter.** Carregue este arquivo para **descobrir** qual doc cobre um"
  echo "> tema (case por \`keywords\`), depois leia só o frontmatter do doc-alvo e pule para a"
  echo "> §seção certa. Protocolo completo em \`.claude/knowledge/shared/rag-retrieval-protocol.md\`."
  echo ">"
  echo "> **Não edite a tabela à mão.** Regenere: \`bash .claude/scripts/gen-index.sh $corpus\`"
  echo ""
  echo "| Contexto | Submódulo | Versão | Status | Keywords | Arquivo |"
  echo "|----------|-----------|--------|--------|----------|---------|"
} > "$tmp"

# Extrai um campo do frontmatter (bloco entre o primeiro e o segundo '---').
field() {
  awk -v key="$2" '
    NR==1 && $0!="---" { exit }
    NR==1 { infm=1; next }
    infm && $0=="---" { exit }
    infm {
      line=$0
      pos=index(line, ":")
      if (pos==0) next
      k=substr(line, 1, pos-1)
      gsub(/^[ \t]+|[ \t]+$/, "", k)
      if (k==key) {
        v=substr(line, pos+1)
        gsub(/^[ \t]+|[ \t]+$/, "", v)
        gsub(/#.*$/, "", v)            # remove comentário inline
        gsub(/^[ \t]+|[ \t]+$/, "", v)
        gsub(/^"|"$/, "", v)           # remove aspas
        print v
      }
    }
  ' "$1"
}

# Coleta as linhas ordenáveis num buffer separado (chave = contexto\tsubmodulo).
rows="$(mktemp)"
trap 'rm -f "$tmp" "$rows"' EXIT

while IFS= read -r file; do
  [ -f "$file" ] || continue
  ctx="$(field "$file" contexto)"
  [ -n "$ctx" ] || continue          # sem frontmatter válido → ignora
  sub="$(field "$file" submodulo)"; [ -n "$sub" ] || sub="n/a"
  ver="$(field "$file" versao)";    [ -n "$ver" ] || ver="1.0"
  sts="$(field "$file" status)";    [ -n "$sts" ] || sts="—"
  kw="$(field "$file" keywords)";   [ -n "$kw" ] || kw="[]"
  printf '%s\t%s\t| %s | %s | %s | %s | %s | %s |\n' \
    "$ctx" "$sub" "$ctx" "$sub" "$ver" "$sts" "$kw" "$file" >> "$rows"
done < <(find "$corpus" -type f -name '*.md' ! -name 'INDEX.md' | LC_ALL=C sort)

# Ordena por contexto+submódulo e descarta as duas colunas-chave.
LC_ALL=C sort "$rows" | cut -f3- >> "$tmp"

mv "$tmp" "$index"
trap 'rm -f "$rows"' EXIT
echo "regenerado: $index ($(grep -c '^| ' "$index" | awk '{print $1-1}') docs)"
