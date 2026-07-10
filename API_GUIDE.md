# Shadow Leveling — Guia de API para Flutter

> Documento gerado para orientar o desenvolvimento do app Flutter com todas as
> funcionalidades e endpoints disponíveis no backend.

---

## Sobre o Projeto Backend

O backend está implementado em **Go** com **PostgreSQL** e **Redis**. Abaixo está
um resumo do que já está **pronto e funcional** para o Flutter consumir:

| Módulo                | O que está pronto                                                                             |
| --------------------- | --------------------------------------------------------------------------------------------- |
| **Autenticação**      | Registro e login (token direto, sem 2FA), perfil/nickname, logout, sessões                    |
| **Tarefas**           | CRUD de tarefas com recorrência, listagem por dia/mês, conclusão                              |
| **Exercícios**        | Catálogo de exercícios com busca e paginação                                                  |
| **Workouts**          | CRUD de treinos, adição/reordenação de exercícios (todos os dias da semana incluindo sáb/dom) |
| **Sessões de Treino** | Registro de sessões, sets por exercício, histórico                                            |
| **Progresso**         | Progresso por exercício ao longo do tempo                                                     |
| **Sessões Perdidas**  | Listagem de treinos não realizados                                                            |
| **Métricas do Dia**   | Dashboard com missões do dia (tarefas + treinos, todos os dias da semana)                     |
| **Nivelamento**       | XP, níveis, ranks (E→S) e streak por conclusão de treino                                      |

---

## Base URL

```
http://<host>:<port>
```

Todas as rotas da API retornam `Content-Type: application/json`.

---

## Autenticação

O backend usa **Bearer Token** (não JWT). O registro e o login retornam o
token **diretamente** em uma única requisição — não há verificação por
e-mail / código 2FA. O token é um hexadecimal de 64 caracteres.

**Header obrigatório nas rotas privadas:**

```
Authorization: Bearer <token>
```

**Formato de erro de autenticação:**

```json
{ "error": "unauthorized" }
```

---

## Códigos de Status HTTP

| Código                      | Quando ocorre                    |
| --------------------------- | -------------------------------- |
| `200 OK`                    | Sucesso em GET/PUT/PATCH         |
| `201 Created`               | Recurso criado                   |
| `204 No Content`            | Sucesso em DELETE/logout         |
| `400 Bad Request`           | Dados de entrada inválidos       |
| `401 Unauthorized`          | Token inválido ou expirado       |
| `403 Forbidden`             | Recurso pertence a outro usuário |
| `404 Not Found`             | Recurso não encontrado           |
| `409 Conflict`              | Email já cadastrado              |
| `500 Internal Server Error` | Erro interno                     |

**Formato padrão de erro:**

```json
{ "error": "mensagem descritiva" }
```

---

---

# MÓDULO 1 — Autenticação (`/auth`)

> Rotas **públicas** não precisam de token. Rotas **privadas** precisam do
> header `Authorization: Bearer <token>`.

---

## 1.1 Registro de Usuário

```
POST /auth/register
```

**Body:**

```json
{
  "email": "user@example.com",
  "password": "minimo8chars"
}
```

**Resposta 201:**

```json
{
  "token": "a1b2c3...64chars",
  "expires_at": "2024-02-01T12:00:00Z"
}
```

> O token é retornado **imediatamente** — não há verificação por e-mail.
> Já pode ser usado no header `Authorization: Bearer <token>`.

**Erros:** `400` (dados inválidos), `409` (email já em uso)

---

## 1.2 Login

```
POST /auth/login
```

**Body:**

```json
{
  "email": "user@example.com",
  "password": "suasenha"
}
```

**Resposta 200:**

```json
{
  "token": "a1b2c3...64chars",
  "expires_at": "2024-02-01T12:00:00Z"
}
```

> O token é retornado **imediatamente** — não há verificação por e-mail / 2FA.

**Erros:** `401` (credenciais inválidas)

---

## 1.3 Usuário Autenticado (privado)

### Obter dados do usuário logado

```
GET /auth/me
```

**Resposta 200:**

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "nickname": "ShadowHunter",
  "created_at": "2024-01-01T00:00:00Z"
}
```

> `nickname` é `null` até o usuário definir um via `PATCH /auth/me`.

---

### Atualizar Perfil (nickname)

```
PATCH /auth/me
```

**Body:**

```json
{ "nickname": "ShadowHunter" }
```

| Campo      | Tipo   | Obrigatório | Valores    |
| ---------- | ------ | ----------- | ---------- |
| `nickname` | string | sim         | 2–30 chars |

**Resposta 200:**

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "nickname": "ShadowHunter",
  "created_at": "2024-01-01T00:00:00Z"
}
```

**Erros:** `400` (nickname inválido)

---

### Logout (revogar sessão atual)

```
POST /auth/logout
```

**Resposta 204** (sem body)

---

### Listar todas as sessões ativas

```
GET /auth/sessions
```

**Resposta 200:**

```json
[
  {
    "id": "uuid",
    "user_agent": "Mozilla/5.0...",
    "created_at": "2024-01-01T00:00:00Z",
    "expires_at": "2024-02-01T00:00:00Z"
  }
]
```

---

### Revogar sessão específica

```
DELETE /auth/sessions/{id}
```

**Parâmetros:** `id` — UUID da sessão  
**Resposta 204** (sem body)  
**Erros:** `404`, `403`

---

---

# MÓDULO 2 — Tarefas (`/tasks`)

> Todas as rotas são **privadas**.
>
> As tarefas suportam recorrência (diária, semanal, mensal, customizada).
> A listagem retorna "ocorrências" — instâncias de uma tarefa em uma data específica.

---

## 2.1 Criar Tarefa

```
POST /tasks
```

**Body:**

```json
{
  "title": "Fazer meditação",
  "description": "10 minutos de meditação guiada",
  "level": "easy",
  "initial_date": "2024-01-01T00:00:00Z",
  "final_date": "2024-12-31T00:00:00Z",
  "recurrence_type": "daily",
  "custom_days_of_week": [],
  "is_optional": false
}
```

| Campo                 | Tipo     | Obrigatório | Valores aceitos                                                              |
| --------------------- | -------- | ----------- | ---------------------------------------------------------------------------- |
| `title`               | string   | sim         | 1–150 chars                                                                  |
| `description`         | string   | não         | 0–1000 chars                                                                 |
| `level`               | string   | sim         | `easy`, `medium`, `hard`, `no_rank`                                          |
| `initial_date`        | datetime | sim         | ISO 8601                                                                     |
| `final_date`          | datetime | sim         | >= `initial_date`                                                            |
| `recurrence_type`     | string   | sim         | `one_time`, `daily`, `weekly`, `monthly`, `custom`                           |
| `custom_days_of_week` | array    | se `custom` | `sunday`, `monday`, `tuesday`, `wednesday`, `thursday`, `friday`, `saturday` |
| `is_optional`         | boolean  | sim         | —                                                                            |

**Resposta 201:** Task completa com `id`

---

## 2.2 Listar Tarefas por Dia (todas)

```
GET /tasks/day?date=2024-01-15
```

**Query Params:** `date` obrigatório, formato `YYYY-MM-DD`

**Resposta 200:**

```json
[
  {
    "id": "uuid",
    "title": "Fazer meditação",
    "description": "...",
    "level": "easy",
    "recurrence_type": "daily",
    "is_optional": false,
    "is_completed": false,
    "occurrence_date": "2024-01-15T00:00:00Z"
  }
]
```

---

## 2.3 Listar Tarefas Não Concluídas de um Dia

```
GET /tasks/uncompleted?date=2024-01-15
```

**Query Params:** `date` obrigatório  
**Resposta 200:** Mesmo formato de `/tasks/day`, filtrado por `is_completed: false`

---

## 2.4 Listar Tarefas do Mês

```
GET /tasks/month?year=2024&month=1
```

**Query Params:** `year` e `month` obrigatórios (month: 1–12)  
**Resposta 200:** Array de TaskOccurrence (todas as ocorrências do mês)

---

## 2.5 Concluir Tarefa

```
PATCH /tasks/{id}/complete
```

**Parâmetros:** `id` — UUID da tarefa  
**Body (opcional):**

```json
{ "date": "2024-01-15T00:00:00Z" }
```

> Se `date` não for enviado, usa a data atual.

**Resposta 200:**

```json
{
  "id": "uuid",
  "title": "Fazer meditação",
  "is_completed": true,
  "occurrence_date": "2024-01-15T00:00:00Z"
}
```

**Erros:** `400` (tarefa não agendada para essa data), `403`, `404`

---

---

# MÓDULO 3 — Exercícios (`/exercises`)

> `GET /exercises` e `GET /exercises/{id}` são **públicos**.
> `POST /exercises` é **privado**.

---

## 3.1 Listar Exercícios (com busca e paginação)

```
GET /exercises?search=agachamento&cursor=abc&limit=20
```

**Query Params:**

| Param    | Obrigatório | Descrição                           |
| -------- | ----------- | ----------------------------------- |
| `search` | não         | Filtro por nome                     |
| `cursor` | não         | Token para próxima página           |
| `limit`  | não         | Itens por página (1–100, padrão 20) |

**Resposta 200:**

```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Agachamento",
      "type": "repetition",
      "unit": "reps",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "cursor": {
    "next_cursor": "eyJpZCI6...base64",
    "has_more": true
  }
}
```

> Use `next_cursor` como `cursor` na próxima requisição para paginar.

---

## 3.2 Obter Exercício por ID

```
GET /exercises/{id}
```

**Resposta 200:**

```json
{
  "id": "uuid",
  "name": "Supino Reto",
  "type": "repetition",
  "unit": "reps",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## 3.3 Criar Exercício (privado)

```
POST /exercises
```

**Body:**

```json
{
  "name": "Rosca Direta",
  "type": "repetition",
  "unit": "reps"
}
```

| Campo  | Tipo   | Obrigatório | Valores                             |
| ------ | ------ | ----------- | ----------------------------------- |
| `name` | string | sim         | 1–100 chars                         |
| `type` | string | sim         | `repetition`, `time`                |
| `unit` | string | sim         | 1–20 chars (ex: "reps", "segundos") |

**Resposta 201:** Exercise completo com `id`

---

---

# MÓDULO 4 — Workouts (`/workouts`)

> Todas as rotas são **privadas**.

---

## 4.1 Listar Workouts do Usuário

```
GET /workouts
```

**Resposta 200:**

```json
[
  {
    "id": "uuid",
    "name": "Treino A - Peito e Tríceps",
    "description": "Foco em peito e tríceps",
    "days_of_week": ["monday", "thursday"],
    "active": true,
    "done_today": false,
    "exercises": [
      {
        "id": "uuid",
        "exercise_id": "uuid",
        "exercise": {
          "id": "uuid",
          "name": "Supino Reto",
          "type": "repetition",
          "unit": "reps"
        },
        "sets": 4,
        "reps_min": 8,
        "reps_max": 12,
        "duration": null,
        "note": "Descanso de 60s",
        "sort_order": 0,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

> `done_today: true` indica que já existe uma sessão **completa** para esse
> workout hoje.

---

## 4.2 Criar Workout

```
POST /workouts
```

**Body:**

```json
{
  "name": "Treino B - Costas e Bíceps",
  "description": "Foco em costas e bíceps",
  "days_of_week": ["tuesday", "friday"]
}
```

| Campo          | Tipo   | Obrigatório | Valores      |
| -------------- | ------ | ----------- | ------------ |
| `name`         | string | sim         | 1–100 chars  |
| `description`  | string | não         | 0–500 chars  |
| `days_of_week` | array  | sim         | mínimo 1 dia |

Dias aceitos: `monday`, `tuesday`, `wednesday`, `thursday`, `friday`, `saturday`, `sunday`

> **Atualização:** sábado (`saturday`) e domingo (`sunday`) agora são suportados também no dashboard de missões do dia (`GET /user-metrics/today`).

**Resposta 201:** Workout criado (sem exercises ainda)

---

## 4.3 Obter Workout por ID

```
GET /workouts/{id}
```

**Resposta 200:** WorkoutDetail com lista de exercises

---

## 4.4 Atualizar Workout

```
PUT /workouts/{id}
```

**Body (todos opcionais):**

```json
{
  "name": "Novo Nome",
  "description": "Nova descrição",
  "days_of_week": ["wednesday", "saturday"],
  "active": false
}
```

**Resposta 200:** Workout atualizado

---

## 4.5 Deletar Workout

```
DELETE /workouts/{id}
```

**Resposta 204** (sem body)

---

## 4.6 Adicionar Exercício ao Workout

```
POST /workouts/{id}/exercises
```

**Body:**

```json
{
  "exercise_id": "uuid-do-exercicio",
  "sets": 4,
  "reps_min": 8,
  "reps_max": 12,
  "duration": null,
  "note": "Executar com carga controlada",
  "sort_order": 0
}
```

| Campo         | Tipo   | Obrigatório | Descrição                               |
| ------------- | ------ | ----------- | --------------------------------------- |
| `exercise_id` | UUID   | sim         | ID do exercício                         |
| `sets`        | int    | sim         | Número de séries (mín 1)                |
| `reps_min`    | int    | não         | Mínimo de repetições                    |
| `reps_max`    | int    | não         | Máximo de repetições                    |
| `duration`    | int    | não         | Duração em segundos (para `type: time`) |
| `note`        | string | não         | Observação (máx 500 chars)              |
| `sort_order`  | int    | sim         | Posição na lista (>= 0)                 |

> Máximo de **50 exercícios** por workout.

**Resposta 201:** WorkoutExercise criado

---

## 4.7 Atualizar Exercício do Workout

```
PUT /workouts/{id}/exercises/{weId}
```

**Body (todos opcionais):**

```json
{
  "sets": 3,
  "reps_min": 10,
  "reps_max": 15,
  "duration": null,
  "note": "Nova observação",
  "sort_order": 1
}
```

**Resposta 200:** WorkoutExercise atualizado

---

## 4.8 Reordenar Exercícios do Workout

```
PATCH /workouts/{id}/exercises/reorder
```

**Body:**

```json
{
  "exercises": [
    { "id": "uuid-workout-exercise-1", "sort_order": 0 },
    { "id": "uuid-workout-exercise-2", "sort_order": 1 },
    { "id": "uuid-workout-exercise-3", "sort_order": 2 }
  ]
}
```

**Resposta 204** (sem body)

---

## 4.9 Remover Exercício do Workout

```
DELETE /workouts/{id}/exercises/{weId}
```

**Resposta 204** (sem body)

---

## 4.10 Histórico de Progresso do Workout

```
GET /workouts/{id}/progress?exercise_id=uuid
```

**Query Params:** `exercise_id` (opcional) — filtra por exercício específico

**Resposta 200:**

```json
[
  {
    "exercise_id": "uuid",
    "exercise_name": "Supino Reto",
    "exercise_type": "repetition",
    "sessions": [
      {
        "date": "2024-01-10T00:00:00Z",
        "best_set": {
          "id": "uuid",
          "session_id": "uuid",
          "exercise_id": "uuid",
          "set_number": 2,
          "reps": 12,
          "weight": 80.0,
          "duration": null,
          "created_at": "2024-01-10T00:00:00Z"
        }
      }
    ]
  }
]
```

> `best_set` é o melhor set daquele exercício nessa sessão (maior volume ou maior duração).

---

---

# MÓDULO 5 — Sessões de Treino (`/workout-sessions`)

> Todas as rotas são **privadas**.
>
> Uma **sessão** representa a realização de um workout em um dia específico.
> Cada sessão contém **sets** — os registros reais de cada série executada.

---

## 5.1 Listar Sessões

```
GET /workout-sessions?workout_id=uuid&from=2024-01-01&to=2024-01-31
```

**Query Params (todos opcionais):**

| Param        | Descrição                 |
| ------------ | ------------------------- |
| `workout_id` | Filtrar por workout       |
| `from`       | Data inicial (YYYY-MM-DD) |
| `to`         | Data final (YYYY-MM-DD)   |

**Resposta 200:**

```json
[
  {
    "id": "uuid",
    "workout_id": "uuid",
    "date": "2024-01-15T00:00:00Z",
    "status": "complete",
    "created_at": "2024-01-15T08:00:00Z",
    "updated_at": "2024-01-15T09:00:00Z"
  }
]
```

Status possíveis: `complete`, `incomplete`, `skipped`

---

## 5.2 Criar Sessão

```
POST /workout-sessions
```

**Body:**

```json
{
  "workout_id": "uuid",
  "date": "2024-01-15T00:00:00Z",
  "status": "incomplete"
}
```

**Resposta 201:** WorkoutSession criado

---

## 5.3 Obter Sessão por ID (com sets)

```
GET /workout-sessions/{id}
```

**Resposta 200:**

```json
{
  "id": "uuid",
  "workout_id": "uuid",
  "date": "2024-01-15T00:00:00Z",
  "status": "complete",
  "created_at": "...",
  "updated_at": "...",
  "sets": [
    {
      "id": "uuid",
      "session_id": "uuid",
      "exercise_id": "uuid",
      "set_number": 1,
      "reps": 10,
      "weight": 80.0,
      "duration": null,
      "created_at": "2024-01-15T08:30:00Z"
    }
  ]
}
```

---

## 5.4 Atualizar Status da Sessão

```
PUT /workout-sessions/{id}
```

**Body:**

```json
{ "status": "complete" }
```

**Resposta 200:** WorkoutSession atualizado

---

## 5.5 Listar Treinos Perdidos

```
GET /workout-sessions/missed?from=2024-01-01&to=2024-01-31
```

**Query Params (opcionais):** `from`, `to` — padrão: últimos 30 dias

**Resposta 200:**

```json
[
  {
    "date": "2024-01-08T00:00:00Z",
    "workout_id": "uuid",
    "workout_name": "Treino A - Peito"
  }
]
```

> Retorna dias em que um workout estava programado mas não foi registrado.

---

## 5.6 Registrar Set em uma Sessão

```
POST /workout-sessions/{id}/sets
```

**Body:**

```json
{
  "exercise_id": "uuid",
  "set_number": 1,
  "reps": 10,
  "weight": 80.5,
  "duration": null
}
```

| Campo         | Tipo  | Obrigatório | Descrição                  |
| ------------- | ----- | ----------- | -------------------------- |
| `exercise_id` | UUID  | sim         | Exercício executado        |
| `set_number`  | int   | sim         | Número da série (>= 1)     |
| `reps`        | int   | não         | Repetições (>= 0)          |
| `weight`      | float | não         | Peso em kg (>= 0)          |
| `duration`    | int   | não         | Duração em segundos (>= 0) |

**Resposta 201:** ExerciseSet criado

---

## 5.7 Atualizar Set

```
PUT /workout-sessions/{id}/sets/{setId}
```

**Body (todos opcionais):**

```json
{
  "reps": 12,
  "weight": 82.5,
  "duration": null
}
```

**Resposta 200:** ExerciseSet atualizado

---

## 5.8 Deletar Set

```
DELETE /workout-sessions/{id}/sets/{setId}
```

**Resposta 204** (sem body)

---

---

# MÓDULO 6 — Métricas do Dia (`/user-metrics`)

> Rota **privada**. Ideal para a **tela home/dashboard** do app.

---

## 6.1 Missões e Progresso de Hoje

```
GET /user-metrics/today
```

**Resposta 200:**

```json
{
  "date": "2024-01-15T00:00:00Z",
  "progress": {
    "total": 5,
    "completed": 2,
    "pending": 3
  },
  "workouts": {
    "progress": {
      "total": 2,
      "completed": 1,
      "pending": 1
    },
    "items": [
      {
        "id": "uuid",
        "name": "Treino A - Peito",
        "description": "Foco em peito e tríceps",
        "is_completed": true
      },
      {
        "id": "uuid",
        "name": "Treino B - Costas",
        "description": "",
        "is_completed": false
      }
    ]
  },
  "tasks": {
    "progress": {
      "total": 3,
      "completed": 1,
      "pending": 2
    },
    "items": [
      {
        "id": "uuid",
        "level": "easy",
        "title": "Fazer meditação",
        "description": "10 minutos",
        "occurrence_date": "2024-01-15T00:00:00Z",
        "is_optional": false,
        "is_completed": true
      }
    ]
  }
}
```

---

---

# MÓDULO 7 — Nivelamento (`/me/level`)

> Rota **privada**. Retorna o progresso de XP, nível e rank do usuário ("caçador").
>
> O nível é **derivado automaticamente** do XP total — não precisa ser gerenciado pelo front.
> XP é concedido toda vez que uma sessão de treino é marcada como `complete`.

---

## 7.1 Obter Nível e XP do Usuário

```
GET /me/level
```

**Resposta 200:**

```json
{
  "level": 7,
  "rank": "D-Rank",
  "total_xp": 4250,
  "xp_into_level": 650,
  "xp_for_next_level": 1300,
  "progress_pct": 50,
  "current_streak": 4
}
```

| Campo               | Tipo   | Descrição                                            |
| ------------------- | ------ | ---------------------------------------------------- |
| `level`             | int    | Nível atual do usuário (começa em 1)                 |
| `rank`              | string | Rank temático derivado do nível (veja tabela abaixo) |
| `total_xp`          | int    | XP total acumulado                                   |
| `xp_into_level`     | int    | XP acumulado dentro do nível atual                   |
| `xp_for_next_level` | int    | XP total necessário para completar o nível atual     |
| `progress_pct`      | int    | Percentual de progresso no nível atual (0–100)       |
| `current_streak`    | int    | Dias consecutivos com pelo menos um treino concluído |

> Usuário sem nenhum treino concluído ainda: retorna `level: 1`, `rank: "E-Rank"`, todos os outros campos zerados.

---

## 7.2 Tabela de Ranks

| Nível   | Rank     |
| ------- | -------- |
| 1 – 4   | `E-Rank` |
| 5 – 9   | `D-Rank` |
| 10 – 19 | `C-Rank` |
| 20 – 29 | `B-Rank` |
| 30 – 49 | `A-Rank` |
| 50+     | `S-Rank` |

---

## 7.3 Como o XP é calculado

| Evento                                    | XP                           |
| ----------------------------------------- | ---------------------------- |
| Concluir um treino (`status: "complete"`) | **+50 XP**                   |
| Bônus de streak (dias consecutivos)       | **+5 XP × streak** (máx +50) |

**Exemplos de XP ganho por treino:**

| Streak atual | Bônus     | Total ganho |
| ------------ | --------- | ----------- |
| 1 dia        | +5        | **55 XP**   |
| 3 dias       | +15       | **65 XP**   |
| 5 dias       | +25       | **75 XP**   |
| 10+ dias     | +50 (cap) | **100 XP**  |

**Regras do streak:**

- Treinar em dias consecutivos incrementa o streak.
- Faltar um dia **reseta** o streak para 1 (sem perda de XP já acumulado).
- Dois ou mais treinos no mesmo dia contam como 1 para o streak.
- O streak usa a **data do treino**, não a hora do request.

---

## 7.4 Fórmula de Nível (para exibição no front)

O front pode calcular localmente para exibir barras de progresso:

```
XP necessário para atingir o nível N = 100 × (N - 1)²
```

| Nível | XP total necessário |
| ----- | ------------------- |
| 1     | 0                   |
| 2     | 100                 |
| 3     | 400                 |
| 5     | 1.600               |
| 10    | 8.100               |
| 20    | 36.100              |

> O endpoint `GET /me/level` já retorna `xp_into_level`, `xp_for_next_level` e `progress_pct` calculados — o front só precisa renderizar.

---

---

# MÓDULO 8 — Utilitários

---

## 8.1 Health Check (público)

```
GET /health
```

**Resposta 200:**

```json
{ "status": "ok" }
```

---

## 8.2 Documentação Interativa (público)

```
GET /docs
```

Retorna interface HTML (Scalar API Reference) com todos os endpoints documentados.

```
GET /docs/openapi.yaml
```

Retorna a especificação OpenAPI 3.0 em YAML.

---

---

# Resumo de Endpoints para o Flutter

| Método   | Endpoint                              | Auth | Descrição                      |
| -------- | ------------------------------------- | ---- | ------------------------------ |
| `POST`   | `/auth/register`                      | —    | Criar conta (retorna token)    |
| `POST`   | `/auth/login`                         | —    | Login (retorna token)          |
| `GET`    | `/auth/me`                            | sim  | Dados do usuário logado        |
| `PATCH`  | `/auth/me`                            | sim  | Atualizar nickname             |
| `POST`   | `/auth/logout`                        | sim  | Logout                         |
| `GET`    | `/auth/sessions`                      | sim  | Listar sessões                 |
| `DELETE` | `/auth/sessions/{id}`                 | sim  | Revogar sessão                 |
| `POST`   | `/tasks`                              | sim  | Criar tarefa                   |
| `GET`    | `/tasks/day`                          | sim  | Tarefas do dia                 |
| `GET`    | `/tasks/uncompleted`                  | sim  | Tarefas pendentes do dia       |
| `GET`    | `/tasks/month`                        | sim  | Tarefas do mês                 |
| `PATCH`  | `/tasks/{id}/complete`                | sim  | Concluir tarefa                |
| `GET`    | `/exercises`                          | —    | Listar exercícios              |
| `GET`    | `/exercises/{id}`                     | —    | Detalhes de um exercício       |
| `POST`   | `/exercises`                          | sim  | Criar exercício                |
| `GET`    | `/workouts`                           | sim  | Listar workouts                |
| `POST`   | `/workouts`                           | sim  | Criar workout                  |
| `GET`    | `/workouts/{id}`                      | sim  | Detalhes do workout            |
| `PUT`    | `/workouts/{id}`                      | sim  | Atualizar workout              |
| `DELETE` | `/workouts/{id}`                      | sim  | Deletar workout                |
| `POST`   | `/workouts/{id}/exercises`            | sim  | Adicionar exercício ao workout |
| `PUT`    | `/workouts/{id}/exercises/{weId}`     | sim  | Atualizar exercício do workout |
| `PATCH`  | `/workouts/{id}/exercises/reorder`    | sim  | Reordenar exercícios           |
| `DELETE` | `/workouts/{id}/exercises/{weId}`     | sim  | Remover exercício do workout   |
| `GET`    | `/workouts/{id}/progress`             | sim  | Histórico de progresso         |
| `GET`    | `/workout-sessions`                   | sim  | Listar sessões de treino       |
| `POST`   | `/workout-sessions`                   | sim  | Criar sessão de treino         |
| `GET`    | `/workout-sessions/{id}`              | sim  | Detalhes da sessão             |
| `PUT`    | `/workout-sessions/{id}`              | sim  | Atualizar status da sessão     |
| `GET`    | `/workout-sessions/missed`            | sim  | Treinos perdidos               |
| `POST`   | `/workout-sessions/{id}/sets`         | sim  | Registrar set                  |
| `PUT`    | `/workout-sessions/{id}/sets/{setId}` | sim  | Atualizar set                  |
| `DELETE` | `/workout-sessions/{id}/sets/{setId}` | sim  | Deletar set                    |
| `GET`    | `/user-metrics/today`                 | sim  | Dashboard do dia               |
| `GET`    | `/me/level`                           | sim  | XP, nível, rank e streak       |
| `GET`    | `/health`                             | —    | Status da API                  |

---

# Fluxo de Telas Sugerido para o Flutter

```
Splash
  └─> Auth (não logado)
        ├─ Tela de Login → POST /auth/login (retorna token direto)
        └─ Tela de Registro → POST /auth/register (retorna token direto)

Home (logado) — GET /user-metrics/today
  ├─ Lista de missões do dia (workouts + tasks)
  └─ Progresso geral (barra de progresso)

Tarefas
  ├─ Calendário mensal → GET /tasks/month
  ├─ Lista do dia → GET /tasks/day
  └─ Criar tarefa → POST /tasks

Treinos
  ├─ Lista de workouts → GET /workouts
  ├─ Detalhe do workout → GET /workouts/{id}
  ├─ Criar workout → POST /workouts
  ├─ Adicionar exercício → POST /workouts/{id}/exercises
  └─ Progresso → GET /workouts/{id}/progress

Sessão de Treino (ao executar um treino)
  ├─ Criar sessão → POST /workout-sessions
  ├─ Para cada série executada → POST /workout-sessions/{id}/sets
  └─ Finalizar → PUT /workout-sessions/{id} (status: complete)

Histórico
  ├─ Sessões passadas → GET /workout-sessions
  ├─ Treinos perdidos → GET /workout-sessions/missed
  └─ Detalhes de uma sessão → GET /workout-sessions/{id}

Perfil / Progresso
  ├─ Dados do usuário → GET /auth/me
  ├─ Editar nickname → PATCH /auth/me
  ├─ Nível, XP e streak → GET /me/level
  ├─ Sessões ativas → GET /auth/sessions
  └─ Logout → POST /auth/logout
```
