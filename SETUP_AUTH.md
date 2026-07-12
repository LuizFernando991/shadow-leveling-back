# Setup de autenticação — backend

O login usa 3 caminhos, todos **sem senha**: Google, Apple e email + código de
6 dígitos. Este guia lista as variáveis de ambiente que o backend precisa e
onde obter cada valor. As chaves ficam em `.env` (veja `.env.example`).

## Resumo

| Env | Obrigatória? | O que é |
|---|---|---|
| `GOOGLE_CLIENT_IDS` | para login Google | CSV das audiences aceitas (client IDs do Google) |
| `APPLE_CLIENT_IDS` | para login Apple | CSV das audiences aceitas (bundle/service IDs) |
| `RESEND_API_KEY` | prod (email) | chave da Resend; vazio em dev usa o DevSender |
| `EMAIL_FROM` | prod (email) | remetente verificado na Resend |

O backend **verifica** os ID tokens que o app obtém; ele não pede token nenhum
ao Google/Apple. Por isso precisa saber quais `aud` (client IDs) aceitar.

## Google

1. [Google Cloud Console](https://console.cloud.google.com) → crie/selecione um projeto.
2. *APIs & Services → OAuth consent screen* → configure (External, nome do app, etc.).
3. *APIs & Services → Credentials → Create credentials → OAuth client ID*. Crie:
   - um client **Web application**;
   - um client **iOS** com bundle `com.jonasgz.shadowleveling`;
   - (se for publicar Android) um client **Android** — não afeta o backend.
4. **`GOOGLE_CLIENT_IDS`** = client id **Web** `,` client id **iOS**.

> Importante: o app (via `@react-native-google-signin`) emite o ID token com
> `aud` = **web client id**. Se o web client id não estiver em `GOOGLE_CLIENT_IDS`,
> todo login Google é rejeitado na checagem de audience.

Exemplo:
```
GOOGLE_CLIENT_IDS=1234-web.apps.googleusercontent.com,1234-ios.apps.googleusercontent.com
```

## Apple

1. [Apple Developer](https://developer.apple.com) (conta paga) → *Certificates, IDs & Profiles → Identifiers*.
2. No App ID `com.jonasgz.shadowleveling`, habilite a capability **Sign in with Apple**.
3. **`APPLE_CLIENT_IDS`** = o bundle id do app (o `aud` do token no login nativo iOS).

```
APPLE_CLIENT_IDS=com.jonasgz.shadowleveling
```

## Email (código de verificação) — Resend

1. [resend.com](https://resend.com) → *API Keys* → crie uma chave → `RESEND_API_KEY`.
2. *Domains* → verifique um domínio (registros DNS) e use um endereço dele em
   `EMAIL_FROM` (ex.: `no-reply@seudominio.com`).

> **Dev sem Resend:** se `RESEND_API_KEY` ficar vazio, o backend usa o DevSender e
> **loga o código no stdout** do servidor. Dá para testar o login por email
> localmente sem provedor: `POST /auth/email/request` → pegue o código no log →
> `POST /auth/email/verify`.

## Teste rápido (local, sem credenciais sociais)

```bash
make infra/up && make run
# em outro terminal:
curl -s localhost:8080/auth/email/request -d '{"email":"you@example.com"}' -H 'Content-Type: application/json'
# pegue o código no log do servidor (DevSender) e:
curl -s localhost:8080/auth/email/verify -d '{"email":"you@example.com","code":"NNNNNN"}' -H 'Content-Type: application/json'
```

Os fluxos Google/Apple só rodam com credenciais reais + o app num dev build
(veja `SETUP_AUTH.md` do frontend).
