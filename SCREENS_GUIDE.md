# Shadow Leveling — Guia de Telas para Design

> Documento para orientar o design do app multiplataforma (iOS, Android, Web).
> Cada tela descreve: propósito, elementos de UI, estados e navegação.

---

## Identidade Visual (sugestão de referência)

- **Tema:** Dark mode como padrão, inspirado em RPG/gamification
- **Estilo:** Interface limpa com elementos de progressão (barras, níveis, XP)
- **Hierarquia de cores sugerida:**
  - Background principal: cinza escuro / quase preto
  - Superfícies/cards: cinza médio escuro
  - Primária/destaque: roxo ou azul elétrico
  - Sucesso/completo: verde
  - Alerta/pendente: amarelo ou laranja
  - Erro: vermelho
  - Níveis de tarefa: fácil → verde, médio → amarelo, difícil → vermelho, sem rank → cinza

---

## Índice de Telas

1. [Splash Screen](#1-splash-screen)
2. [Onboarding](#2-onboarding)
3. [Login — Inserir Credenciais](#3-login--inserir-credenciais)
4. [Login — Verificar Código](#4-login--verificar-código)
5. [Registro — Criar Conta](#5-registro--criar-conta)
6. [Registro — Verificar Código](#6-registro--verificar-código)
7. [Home / Dashboard do Dia](#7-home--dashboard-do-dia)
8. [Tarefas — Lista do Dia](#8-tarefas--lista-do-dia)
9. [Tarefas — Calendário Mensal](#9-tarefas--calendário-mensal)
10. [Tarefas — Criar / Editar Tarefa](#10-tarefas--criar--editar-tarefa)
11. [Workouts — Lista de Treinos](#11-workouts--lista-de-treinos)
12. [Workouts — Detalhe do Treino](#12-workouts--detalhe-do-treino)
13. [Workouts — Criar / Editar Treino](#13-workouts--criar--editar-treino)
14. [Exercícios — Catálogo](#14-exercícios--catálogo)
15. [Exercícios — Detalhe / Criar Exercício](#15-exercícios--detalhe--criar-exercício)
16. [Sessão de Treino — Em andamento](#16-sessão-de-treino--em-andamento)
17. [Sessão de Treino — Registrar Set](#17-sessão-de-treino--registrar-set)
18. [Sessão de Treino — Resumo / Finalizar](#18-sessão-de-treino--resumo--finalizar)
19. [Histórico — Lista de Sessões](#19-histórico--lista-de-sessões)
20. [Histórico — Detalhe da Sessão](#20-histórico--detalhe-da-sessão)
21. [Histórico — Treinos Perdidos](#21-histórico--treinos-perdidos)
22. [Progresso — Gráfico por Exercício](#22-progresso--gráfico-por-exercício)
23. [Perfil — Dados do Usuário](#23-perfil--dados-do-usuário)
24. [Perfil — Sessões Ativas](#24-perfil--sessões-ativas)
25. [Navegação Principal (Bottom Nav / Drawer)](#25-navegação-principal)

---

---

## 1. Splash Screen

**Propósito:** Tela inicial enquanto o app verifica autenticação.

**Elementos:**

- Logo / nome do app centralizado
- Animação de loading sutil (spinner ou logo animado)
- Background escuro com efeito de gradiente

**Comportamento:**

- Verifica se há token salvo localmente
- Se token válido → redireciona para **Home**
- Se não → redireciona para **Onboarding** ou **Login**

**Estados:** carregando

---

## 2. Onboarding

**Propósito:** Apresentar o app para novos usuários antes de logar.

**Elementos:**

- 3 slides com ilustração + título + subtítulo:
  - Slide 1: "Suas missões diárias" — gerencie tarefas como um RPG
  - Slide 2: "Treinos organizados" — monte seus workouts e registre séries
  - Slide 3: "Evolua todo dia" — acompanhe seu progresso e não perca treinos
- Indicadores de slide (dots)
- Botão "Próximo" / "Começar"
- Link "Já tenho conta" (vai para Login)

**Navegação:**

- Último slide → botão "Começar" → **Registro**
- Link "Já tenho conta" → **Login**

---

## 3. Login — Inserir Credenciais

**Propósito:** Autenticar usuário existente com e-mail e senha.

**Elementos:**

- Logo / título "Entrar"
- Campo de texto: E-mail (tipo email, teclado email)
- Campo de texto: Senha (oculta, com botão de olho para revelar)
- Botão primário "Enviar código"
- Link "Esqueci minha senha" (futuro)
- Link "Criar conta" → Registro

**Estados:**

- Padrão (campos vazios)
- Preenchendo
- Carregando (botão desabilitado + spinner)
- Erro inline (ex: "E-mail ou senha incorretos")

**Navegação:**

- Sucesso → **Login — Verificar Código** (passando o e-mail)

---

## 4. Login — Verificar Código

**Propósito:** Confirmar identidade via código de 6 dígitos enviado por e-mail.

**Elementos:**

- Título "Verifique seu e-mail"
- Subtítulo com o e-mail mascarado (ex: `jo***@gmail.com`)
- Campo de código: 6 caixas individuais (OTP input), teclado numérico
- Botão primário "Verificar"
- Link "Reenviar código" (com countdown de 60s antes de habilitar)
- Indicador de expiração do código (ex: "Código válido por 10 minutos")

**Estados:**

- Aguardando input
- Preenchendo (foco automático no próximo campo)
- Carregando
- Erro (ex: "Código inválido ou expirado" — com shake animation nos campos)
- Rate limit (ex: "Muitas tentativas. Aguarde 1 hora.")

**Navegação:**

- Sucesso → **Home**

---

## 5. Registro — Criar Conta

**Propósito:** Criar nova conta com e-mail e senha.

**Elementos:**

- Título "Criar conta"
- Campo de texto: E-mail
- Campo de texto: Senha (com indicador de força)
- Campo de texto: Confirmar senha
- Requisitos de senha visíveis (mínimo 8 caracteres)
- Botão primário "Criar conta"
- Link "Já tenho conta" → Login

**Estados:**

- Padrão
- Preenchendo (validação em tempo real)
- Carregando
- Erro inline (ex: "E-mail já cadastrado")

**Navegação:**

- Sucesso → **Registro — Verificar Código**

---

## 6. Registro — Verificar Código

**Propósito:** Confirmar e-mail do novo usuário.

> Mesma estrutura da tela **4. Login — Verificar Código**, com título
> "Confirme seu e-mail".

**Navegação:**

- Sucesso → **Home** (primeira vez → pode mostrar tutorial rápido)

---

## 7. Home / Dashboard do Dia

**Propósito:** Tela principal do app. Visão geral das missões e progresso do dia.

**Elementos:**

### Header

- Saudação personalizada (ex: "Bom dia, Jonas!")
- Data atual (ex: "Quinta, 15 de janeiro")
- Avatar / ícone de perfil (toca → **Perfil**)

### Card de Progresso Geral

- Barra de progresso circular
- Texto: "X de Y treinos concluídos na semana"
- Percentual de conclusão

### Card de Nivelamento

- mostrando o rank e ao lado o quanto falta para o proximo nivel: "RANK: "x" 1200/2000 XP"
- Barra de progresso linear para o proximo nivel

### Seção: Treinos de Hoje

- Título "Treinos" + badge com contagem (ex: "1/2")
- Lista de cards de workout:
  - Nome do treino
  - Descrição curta
  - Status: ✓ concluído (verde) ou pendente (cinza)
  - Botão "Iniciar" (se pendente) → **Sessão de Treino**
  - Toque → **Detalhe do Workout**

### Estado vazio

- Ilustração + texto "Nenhum treino para hoje"
- Botão "Criar treino"

**Navegação:**

- Bottom navigation bar (fixo)
- FAB ou ícone de "+" para ação rápida

---

## 8. Tarefas — Lista do Dia

**Propósito:** Visualizar e gerenciar todas as tarefas de um dia específico.

**Elementos:**

### Header

- Seletor de data: setas para anterior/próximo dia + data atual clicável
- Filtro rápido: "Todas" | "Pendentes" | "Concluídas" (chips/tabs)

### Lista de Tarefas

- Card por tarefa:
  - Ícone de nível com cor (fácil=verde, médio=amarelo, difícil=vermelho, sem rank=cinza)
  - Título (riscado se concluída)
  - Descrição truncada (1 linha)
  - Badge "opcional" se aplicável
  - Checkbox ou botão de check
  - Swipe para ações (opcional): concluir / editar
- Toque no card → modal ou tela de detalhe da tarefa

### Botão flutuante (FAB)

- Ícone "+" → **Criar Tarefa**

### Estado vazio

- Ilustração + "Nenhuma tarefa para este dia"

---

## 9. Tarefas — Calendário Mensal

**Propósito:** Visualizar distribuição de tarefas no mês.

**Elementos:**

### Header

- Seletor de mês/ano com setas (anterior/próximo)

### Calendário

- Grade mensal (7 colunas)
- Cada dia:
  - Número do dia
  - Indicadores coloridos (dots) representando tarefas pendentes/concluídas
  - Destaque no dia selecionado
  - Destaque no dia de hoje

### Lista do Dia Selecionado

- Ao tocar em um dia → mostra lista de tarefas daquele dia abaixo do calendário (ou em panel deslizante)
- Mesmo formato da lista do dia (**tela 8**)

---

## 10. Tarefas — Criar / Editar Tarefa

**Propósito:** Formulário para criar ou editar uma tarefa.

**Elementos:**

### Header

- Título "Nova tarefa" ou "Editar tarefa"
- Botão "Cancelar" (esquerda) e "Salvar" (direita)

### Formulário

**Título**

- Campo de texto obrigatório (máx 150 chars)
- Contador de caracteres

**Descrição**

- Campo de texto multilinha opcional (máx 1000 chars)

**Nível / Dificuldade**

- Seletor visual com 4 opções em chips/cards:
  - Sem rank (cinza)
  - Fácil (verde)
  - Médio (amarelo)
  - Difícil (vermelho)

**Período**

- Date picker: Data inicial
- Date picker: Data final

**Recorrência**

- Seletor: Uma vez | Diário | Semanal | Mensal | Customizado
- Se "Customizado": grade de dias da semana para selecionar múltiplos (Dom/Seg/Ter/Qua/Qui/Sex/Sáb)

**Opcional**

- Toggle switch "Tarefa opcional"

**Estados:**

- Padrão / preenchendo
- Carregando (ao salvar)
- Erro de validação inline

---

## 11. Workouts — Lista de Treinos

**Propósito:** Visualizar todos os treinos criados pelo usuário.

**Elementos:**

### Header

- Título "Meus Treinos"
- Botão "+" (criar novo) → **Criar Workout**

### Lista de Workout Cards

- Card por treino:
  - Nome do treino (destaque)
  - Descrição curta (1 linha, truncada)
  - Dias da semana em chips (ex: "Seg Qui")
  - Número de exercícios (ex: "8 exercícios")
  - Badge de status: "Ativo" (verde) ou "Inativo" (cinza)
  - Badge "Feito hoje" se `done_today: true` (cor primária)
  - Botão "Iniciar treino" (se hoje for dia programado e não foi feito)

### Estado vazio

- Ilustração + "Você ainda não tem treinos"
- Botão "Criar primeiro treino"

**Navegação:**

- Toque no card → **Detalhe do Workout**
- Botão "Iniciar treino" → **Sessão de Treino — Em andamento**

---

## 12. Workouts — Detalhe do Treino

**Propósito:** Ver todos os exercícios de um treino e iniciar sessão.

**Elementos:**

### Header

- Nome do treino
- Menu (⋮): Editar / Deletar / Ativar-Desativar
- Botão "Iniciar Treino" (destaque, fixo no bottom ou no header)

### Informações do Treino

- Descrição
- Dias da semana
- Status (ativo/inativo)
- Badge "Feito hoje" se aplicável

### Lista de Exercícios

- Card por exercício (na ordem `sort_order`):
  - Número de ordem
  - Nome do exercício
  - Tipo: repetição ou tempo
  - Séries × reps (ex: "4 × 8–12 reps") ou duração (ex: "3 × 45 seg")
  - Observação/nota (se houver)
  - Ações: editar (lápis) / remover (lixeira)
  - Drag handle para reordenar (arrastar)

### Botão Adicionar Exercício

- Botão ao final da lista → **Catálogo de Exercícios** (para escolher e adicionar)

### Seção Progresso

- Link "Ver progresso" → **Progresso por Exercício**

---

## 13. Workouts — Criar / Editar Treino

**Propósito:** Formulário para criar ou editar um treino.

**Elementos:**

### Header

- Título "Novo Treino" ou "Editar Treino"
- Botão "Cancelar" e "Salvar"

### Formulário

**Nome**

- Campo de texto obrigatório (máx 100 chars)

**Descrição**

- Campo de texto opcional (máx 500 chars)

**Dias da Semana**

- Grade de 7 chips selecionáveis (Dom, Seg, Ter, Qua, Qui, Sex, Sáb)
- Mínimo 1 selecionado
- Destaque visual nos selecionados

**Ativo** (apenas ao editar)

- Toggle switch

---

## 14. Exercícios — Catálogo

**Propósito:** Navegar pelo catálogo de exercícios disponíveis e adicionar ao workout.

**Elementos:**

### Header

- Título "Exercícios"
- Campo de busca (search bar) sempre visível
- Botão "Criar exercício" (ícone "+") → **Criar Exercício**

### Filtros

- Chips de tipo: "Todos" | "Repetições" | "Tempo"

### Lista de Exercícios

- Card compacto por exercício:
  - Nome
  - Tipo (chip pequeno: "Repetições" ou "Tempo")
  - Unidade (ex: "reps", "segundos")
  - Botão "Adicionar" (se aberto como modal de seleção para workout)
  - Toque → **Detalhe do Exercício**

### Paginação

- Scroll infinito (carrega mais ao chegar no fim da lista)

### Estado vazio (sem resultados na busca)

- "Nenhum exercício encontrado para '[busca]'"
- Sugestão: "Criar exercício com esse nome"

---

## 15. Exercícios — Detalhe / Criar Exercício

**Propósito:** Ver detalhes de um exercício ou criar novo.

### Detalhe do Exercício

**Elementos:**

- Nome do exercício (título)
- Tipo: Repetições ou Tempo
- Unidade (ex: "reps", "kg", "segundos")
- Data de criação
- Botão "Adicionar ao treino" (abre seletor de workout)

### Criar Exercício (formulário)

**Elementos:**

- Header: "Novo Exercício" + Cancelar / Salvar
- Campo: Nome (obrigatório, máx 100 chars)
- Seletor de Tipo: "Repetições" ou "Tempo" (radio/toggle)
- Campo: Unidade (máx 20 chars, ex: "reps", "seg", "km")

---

## 16. Sessão de Treino — Em andamento

**Propósito:** Tela principal durante a execução do treino. Registrar sets de cada exercício.

**Elementos:**

### Header (fixo)

- Nome do workout
- Cronômetro da sessão (tempo decorrido)
- Botão "Encerrar" → modal de confirmação → **Resumo / Finalizar**

### Barra de Progresso

- Exercício X de Y (ex: "Exercício 2 de 6")
- Barra linear de progresso

### Card do Exercício Atual

- Nome do exercício
- Meta: "4 séries × 8–12 reps" (ou duração)
- Nota/observação do exercício

### Lista de Sets Registrados

- Linha por set:
  - Número da série (ex: "Série 1")
  - Campos: Peso (kg) + Reps — ou — Duração (seg)
  - Status: registrado (verde) / não registrado (cinza)
  - Ação: editar / deletar

### Botão "+ Registrar Próxima Série"

- Abre **modal de registro de set** (tela 17)

### Timer de Descanso (opcional)

- Após registrar um set → mostra contador regressivo de descanso
- Botão para pular o descanso

### Navegação entre Exercícios

- Setas ou swipe para ir para próximo / anterior exercício
- Ou lista lateral de exercícios (drawer)

---

## 17. Sessão de Treino — Registrar Set

**Propósito:** Bottom sheet / modal para registrar uma série.

**Elementos:**

- Título: "Série X — [Nome do Exercício]"
- Para exercícios de **repetição**:
  - Campo: Peso (kg) — teclado numérico decimal
  - Campo: Repetições — teclado numérico
- Para exercícios de **tempo**:
  - Campo: Duração (segundos) — ou timer integrado
- Botão "Salvar Série"
- Botão "Cancelar"

**Comportamento:**

- Pré-preenche com os valores da série anterior (se houver)
- Validação: pelo menos um campo preenchido

---

## 18. Sessão de Treino — Resumo / Finalizar

**Propósito:** Mostrar resumo do treino realizado e permitir finalizar.

**Elementos:**

### Header

- Título "Resumo do Treino"
- Ícone de troféu / conquista

### Estatísticas Gerais

- Duração total da sessão
- Total de séries registradas
- Total de exercícios realizados

### Lista de Exercícios com Sets

- Por exercício:
  - Nome
  - Resumo: melhor set (maior peso/reps ou maior duração)
  - Todas as séries registradas em formato compacto (ex: "80kg × 10, 80kg × 8, 75kg × 12")

### Seletor de Status Final

- Radio ou toggle: "Completo" | "Incompleto" | "Pulado"
- Padrão: "Completo"

### Botão "Finalizar Treino"

- Salva o status da sessão
- Navega para **Home** ou **Histórico**

---

## 19. Histórico — Lista de Sessões

**Propósito:** Ver todas as sessões de treino passadas.

**Elementos:**

### Header

- Título "Histórico"
- Filtros: seletor de período (últimos 7 dias, 30 dias, 3 meses, personalizado)
- Filtro por workout (dropdown)

### Lista de Sessões

- Card por sessão:
  - Nome do workout
  - Data (ex: "Seg, 13 jan")
  - Status: badge colorido ("Completo" verde / "Incompleto" amarelo / "Pulado" cinza)
  - Quantidade de séries registradas
  - Duração (se disponível)
- Agrupado por semana (cabeçalho de semana)

### Estado vazio

- "Nenhuma sessão registrada"

**Navegação:**

- Toque na sessão → **Detalhe da Sessão**

---

## 20. Histórico — Detalhe da Sessão

**Propósito:** Ver todos os sets registrados em uma sessão específica.

**Elementos:**

### Header

- Nome do workout
- Data
- Status (badge)
- Duração total

### Lista de Exercícios

- Por exercício:
  - Nome do exercício
  - Tabela de sets:
    - Número da série | Peso | Reps | ou Duração
  - Melhor set destacado

### Ação

- Botão "Editar status" (alterar entre completo/incompleto/pulado)

---

## 21. Histórico — Treinos Perdidos

**Propósito:** Ver dias em que treinos programados não foram realizados.

**Elementos:**

### Header

- Título "Treinos Perdidos"
- Seletor de período (padrão: últimos 30 dias)

### Lista de Sessões Perdidas

- Card por item:
  - Data (ex: "Terça, 7 jan")
  - Nome do treino que deveria ter sido feito
  - Botão "Registrar agora" (cria sessão retroativa com status "skipped" ou "incomplete")

### Estado vazio (bom estado)

- Ícone de conquista + "Nenhum treino perdido nesse período!"

---

## 22. Progresso — Gráfico por Exercício

**Propósito:** Visualizar evolução de um exercício ao longo do tempo.

**Elementos:**

### Header

- Título "Progresso"
- Seletor de Workout (dropdown)

### Seletor de Exercício

- Lista ou dropdown de exercícios do workout selecionado
- Toque em um exercício → carrega gráfico

### Gráfico de Evolução

- Linha ou barras mostrando o melhor set por sessão ao longo do tempo
- Eixo X: datas das sessões
- Eixo Y: peso (para repetições) ou duração (para tempo)
- Ponto clicável: mostra detalhes do melhor set naquela data

### Tabela de Histórico

- Lista de sessões com o melhor set:
  - Data | Série | Peso/Reps | Duração

### Estado vazio

- "Nenhum dado registrado ainda para este exercício"

---

## 23. Perfil — Dados do Usuário

**Propósito:** Ver e gerenciar informações da conta.

**Elementos:**

### Header

- Título "Perfil"

### Avatar / Identidade

- Inicial do nome / avatar placeholder
- E-mail do usuário
- Data de criação da conta (membro desde...)

### Opções de Menu (lista)

- "Sessões ativas" → **Sessões Ativas** (tela 24)
- "Notificações" (futuro)
- "Tema" (dark/light — futuro)
- "Sobre o app" (versão, termos)

### Botão de Logout

- Botão vermelho / destrutivo "Sair da conta"
- Confirmação via dialog antes de executar

---

## 24. Perfil — Sessões Ativas

**Propósito:** Visualizar e revogar sessões abertas em outros dispositivos.

**Elementos:**

### Header

- Título "Sessões Ativas"
- Botão "Revogar todas" (cuidado — dialog de confirmação)

### Lista de Sessões

- Card por sessão:
  - Ícone de dispositivo (deducido do `user_agent`)
  - Texto do `user_agent` truncado (ex: "Chrome em Windows")
  - Data de criação
  - Data de expiração
  - Badge "Esta sessão" para a sessão atual
  - Botão "Revogar" (não aparece na sessão atual)

### Estado vazio

- "Nenhuma outra sessão ativa"

---

## 25. Navegação Principal

**Propósito:** Estrutura de navegação que conecta todas as telas.

### Bottom Navigation Bar (mobile)

5 itens:

| Ícone            | Label     | Tela              |
| ---------------- | --------- | ----------------- |
| Casa / Home      | Hoje      | Home / Dashboard  |
| Checkbox / Lista | Tarefas   | Lista do Dia      |
| Haltere / Gym    | Treinos   | Lista de Workouts |
| Gráfico / Barra  | Progresso | Progresso         |
| Pessoa           | Perfil    | Perfil            |

### Drawer / Side Nav (tablet / web)

- Mesmos itens + logo no topo
- Largura fixa na lateral esquerda

---

## Modais e Bottom Sheets Recorrentes

| Modal                         | Onde é usado                            |
| ----------------------------- | --------------------------------------- |
| **Confirmação de exclusão**   | Deletar workout, revogar sessão, logout |
| **Seletor de data**           | Criar tarefa, filtros de histórico      |
| **Seletor de dias da semana** | Criar tarefa (custom), criar workout    |
| **Registrar Set**             | Sessão de Treino em andamento           |
| **Timer de descanso**         | Após registrar set                      |
| **Adicionar ao workout**      | Detalhe do exercício                    |
| **Filtros**                   | Histórico de sessões, exercícios        |

---

## Estados Globais de UI

Toda tela deve contemplar os seguintes estados:

| Estado                   | Descrição               | UI sugerida                                                |
| ------------------------ | ----------------------- | ---------------------------------------------------------- |
| **Carregando**           | Requisição em andamento | Skeleton loaders ou shimmer nos cards                      |
| **Vazio**                | Sem dados retornados    | Ilustração + texto + CTA (botão de criar)                  |
| **Erro de rede**         | Sem conexão ou erro 5xx | Banner no topo + botão "Tentar novamente"                  |
| **Erro de autenticação** | Token expirado (401)    | Redireciona para Login automaticamente                     |
| **Sucesso**              | Ação completada         | Snackbar / toast de confirmação (ex: "Treino finalizado!") |

---

## Notações de Gamificação (sugestão de design)

O app tem uma proposta de gamificação (Shadow Leveling). Elementos que podem
enriquecer o design:

- **Missões** ao invés de "tarefas"
- **Nível de dificuldade** representado com ícones estilo RPG (espada, escudo, crânio)
- **Streak** — sequência de dias com missões completas (futuro endpoint)
- **Animação de conclusão** ao completar todas as missões do dia
- **Progress ring** animado na home
- **Sons e haptics** ao completar tarefas (opcional no design)
