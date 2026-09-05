# zhuri

Corretor de encoding para ficheiros de configuração. Não formata YAML, não valida esquema - deteta bytes que não são UTF-8, descobre de que encoding vieram, e converte.

## O problema

Se já apanhaste isto, vais reconhecer na hora:

```
err: yaml: invalid trailing UTF-8 octet
```

A primeira reação de toda a gente é abrir o ficheiro, olhar para a indentação, contar espaços, duvidar da própria sanidade. Mas o YAML está bem. O problema é que alguém - normalmente o Excel, ou um editor com o locale errado, ou um `scp` feito a partir do Windows sem pensar duas vezes - gravou o ficheiro em Windows-1252 em vez de UTF-8. `descrição` virou `descri\xe7\xe3o`. `região` virou `regi\xe3o`. Cada acento português é um ou mais bytes que, interpretados como UTF-8, simplesmente não fazem sentido, e o parser da tua ferramenta de configuração morre a meio.

O `yamllint` não te ajuda aqui - ele só te diz que o ficheiro é inválido, que tu já sabias. O zhuri lê os bytes, percebe que aquilo é Windows-1252 (ou ISO-8859-1, ou UTF-16 com BOM), mostra-te exatamente onde estava o problema, e - se pedires - corrige.

## Como se usa

```
$ zhuri config/app.yaml
app.yaml
  não é UTF-8 válido - detetado Windows-1252 (confiança 95%)
  L1  offset 6   descri\xe7\xe3o: "Servi\xe7o de entrega"
                       ^^^^^^^^         ^^^^
  L3  offset 49   regi\xe3o: sudeste
                      ^^^^
  2 linhas afetadas → usa --write para corrigir
```

Sem flags, o zhuri nunca escreve nada - é sempre pré-visualização. Só quando confias no que viste é que passas `--write`:

```
$ zhuri --write config/app.yaml
app.yaml
  não é UTF-8 válido - detetado Windows-1252 (confiança 95%)
  L1  offset 6   descri\xe7\xe3o: "Servi\xe7o de entrega"
                       ^^^^^^^^         ^^^^
  L3  offset 49   regi\xe3o: sudeste
                      ^^^^
  2 linhas afetadas → corrigido para UTF-8

$ cat config/app.yaml
descrição: "Serviço de entrega"
chave: valor
região: sudeste
```

Correr o zhuri outra vez sobre um ficheiro já corrigido não faz nada - ele valida UTF-8 primeiro e, se já estiver tudo bem, não toca. Isso é deliberado: quero poder meter isto num pre-commit hook sem medo de que ele ande a reescrever ficheiros a cada `git commit`.

A dor a sério aparece quando tens uma pasta `config/` inteira com dezenas de ficheiros deste tipo, alguns bons, alguns não. Para isso há `-r`:

```
$ zhuri -r ./config/
```

## Instalação

```
git clone <este-repositório>
cd zhuri
make build
sudo make install
```

Isto compila um binário estático (`CGO_ENABLED=0`) e copia-o para `/usr/local/bin/zhuri`. Estático quer dizer sem dependência de libc - o mesmo binário corre em Debian, Alpine, Fedora, o que for, sem te preocupares com versões de glibc.

Sem privilégios de root, instala num prefixo teu:

```
make install PREFIX=$HOME/.local
```

(garante que `$HOME/.local/bin` está no `PATH`.) Para desinstalar, `make uninstall` com o mesmo `PREFIX` que usaste.

Se só quiseres o binário sem o instalar em lado nenhum, `make build` deixa-o em `./zhuri`. E se tiveres o Go instalado e preferires o caminho mais direto, `go install ./cmd/zhuri` também funciona a partir da raiz do repositório.

## Flags

```
  -w, --write        aplica as correções (por defeito só pré-visualiza)
      --from <enc>   força o encoding de origem (windows-1252, iso-8859-1, utf-16, ...)
  -r, --recursive    percorre pastas recursivamente
      --backup       cria .bak antes de escrever
      --strip-bom    remove BOM de UTF-8 ao gravar
      --ext <lista>  extensões a processar (defeito: yaml,yml,json,xml,csv,env,properties,toml,ini,txt)
      --max-size <MB> limite de tamanho por ficheiro em MB (defeito: 50)
      --quiet        só imprime ficheiros com problema
      --no-color     desliga cores
  -v, --version
  -h, --help
```

O `--from` existe para quando a deteção automática erra - e vai errar, de vez em quando. Windows-1252 e ISO-8859-1 partilham praticamente todos os bytes acima de 0xA0, por isso um ficheiro puramente latino sem nenhum byte na gama 0x80–0x9F é ambíguo por natureza; o zhuri assume Windows-1252 porque é de longe o caso mais comum em ficheiros gravados no Windows em português, mas se souberes que não é isso, força com `--from iso-8859-1`.

Cores desligam-se sozinhas quando a saída não é um terminal, ou quando `NO_COLOR` está definida - nunca vais ver códigos ANSI a poluir um pipe ou um log de CI.

Códigos de saída: `0` nada a fazer ou tudo corrigido, `1` ficaram problemas por corrigir (normalmente porque correste sem `--write`), `2` erro de execução (caminho inexistente, flag inválida, etc.).

## Segurança

Isto lê e reescreve ficheiros de configuração de outras pessoas, por isso fui cuidadoso com um conjunto de coisas que considero não negociáveis:

- Nunca segue symlinks - nem como argumento direto, nem dentro de uma pasta percorrida com `-r`. São reportados como ignorados, não seguidos.
- Ficheiros especiais (pipes, sockets, devices) são ignorados; só ficheiros regulares são tocados.
- Há um limite de tamanho (50 MB por defeito, ajustável com `--max-size`) verificado antes de ler o ficheiro inteiro para memória.
- Deteta binários (byte nulo, ou densidade alta de bytes de controlo) e recusa-se a tocar-lhes - transcodificar um binário não corrige nada, destrói.
- A escrita é atómica: grava para um ficheiro temporário na mesma pasta e só depois faz `rename` por cima do original. Um crash a meio nunca deixa o ficheiro pela metade.
- As permissões do ficheiro original são preservadas.
- Um ficheiro problemático nunca aborta o lote inteiro - o erro fica reportado e o zhuri segue para o próximo.

## Limitações

A deteção de encoding é heurística, não é magia. Quando não há nenhum byte na gama 0x80–0x9F, Windows-1252 e ISO-8859-1 são indistinguíveis a partir dos bytes sozinhos - o zhuri escolhe Windows-1252 por ser o caso mais frequente, mas pode estar errado no teu caso específico. Usa `--from` quando isso acontecer.

Também não tenta adivinhar Shift-JIS, GBK, ou qualquer encoding fora da família latina - o alvo é explicitamente o cenário de acentos portugueses gravados no Windows. Se o teu problema é outro, esta ferramenta provavelmente não é a certa.

E se um ficheiro foi genuinamente truncado (falta o fim, não é só o encoding errado), o zhuri deteta a sequência UTF-8 incompleta no fim e corta-a para deixar o resto do ficheiro válido - mas o conteúdo que faltava continua perdido. Não há como recuperar dados que nunca chegaram a ser escritos.

## Hook de pre-commit / CI

Localmente, para nunca mais deixar um acento partido entrar num commit:

```bash
#!/bin/sh
# .git/hooks/pre-commit
zhuri -r config/ || {
  echo "há ficheiros com encoding errado em config/ - corre 'zhuri -r --write config/'"
  exit 1
}
```

Em CI, o mesmo comando funciona sem alterações: sem `--write`, o zhuri só verifica, e o código de saída `1` falha o passo quando encontra algo por corrigir.

## Testes

```
go test ./...
go test ./... -bench=. -benchmem
```

ou, com o Makefile: `make check` (vet + gofmt + testes) e `make bench`.

## Licença

MIT. Ver [LICENSE](LICENSE).
