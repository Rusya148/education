# DevOps Roadmap: Minion Bank (Go + React)

**Роль:** Senior DevOps Engineer / Инфраструктурный архитектор
**Стек:** Go, React, PostgreSQL, Kafka, Terraform, Ansible, GitLab CI, Kubernetes, Vault, Prometheus, Grafana, Loki.
**Окружение:** Ubuntu, bash.

Этот план разбит на 8 логических этапов, двигаясь от базовой инфраструктуры до продвинутой автоматизации (Helm + GitOps) и наблюдаемости.

---

## Неделя 1: Infrastructure as Code (Terraform) и базовая инфраструктура

**Что нужно изучить:**
- Основы Terraform (состояния, провайдеры, модули).
- Использование локальных k8s-кластеров через `kind` (Kubernetes in Docker) или `k3d`.
- Базовый Ansible (в контексте первоначальной подготовки ОС, так как мы будем деплоить на Ubuntu).

**Что конкретно сделать в коде:**
1. **Подготовка Ubuntu:**
   В будущем мы автоматизируем это через Ansible плейбук, но для начала установи нужные пакеты:
   ```bash
   sudo apt update && sudo apt install -y docker.io ansible
   # Остальные утилиты (terraform, kubectl, helm, kind) мы установим через Ansible чуть позже
   ```
   *Не забудь добавить юзера в группу docker:* `sudo usermod -aG docker $USER` (и перезайди).

2. **Ansible (Опционально для локалки):**
   Напиши простенький playbook (`ansible/setup-node.yml`), который ставит Docker и базовые системные пакеты. Это хороший фундамент для переезда на облачные VPS.

3. **Terraform (Разворачиваем локальный кластер):**
   Вместо того чтобы руками выполнять `kind create cluster`, опиши это терраформом. Существует провайдер `tehcyx/kind`.
   
   *Пример структуры файлов `terraform/`:*
   ```text
   terraform/
   ├── main.tf          # Описание провайдера и ресурса kind_cluster
   ├── variables.tf     # Переменные (версия k8s, имя)
   ├── outputs.tf       # Вывод пути к сгенерированному kubeconfig
   └── providers.tf     # Блок required_providers
   ```

**Как проверить, что это работает:**
- Находясь в директории `terraform/`, выполни команды (используя `fish`):
  ```fish
  terraform init
  terraform apply -auto-approve
  ```
- Проверь кластер: `kubectl cluster-info` и `kubectl get nodes`. Ноды должны быть в статусе `Ready`.

---

## Неделя 2: Оркестрация приложений (Kubernetes)

**Что нужно изучить:**
- Базовые объекты k8s: `Pod`, `Deployment`, `Service`, `Ingress`, `ConfigMap`, `Secret`.
- Работа с пакетным менеджером Helm (например, для NGINX Ingress).

**Что конкретно сделать в коде:**
1. Разверни NGINX Ingress Controller для обработки входящего трафика внутрь кластера.
2. Подними **PostgreSQL** и **Kafka** локально через `docker-compose.yml`, чтобы они работали надежно вне Kubernetes.
3. Напиши манифесты для своих приложений:
   - `kubernetes/backend/`: `deployment.yaml`, `service.yaml`, `ingress.yaml`. В переменных укажи IP-адрес хост-машины, чтобы под достучался к БД.
   - `kubernetes/frontend/`: `deployment.yaml`, `service.yaml`, `ingress.yaml` (или настрой Nginx Ingress отдавать статику).

**Как проверить, что это работает:**
- `docker-compose ps` покажет, что БД и Kafka работают вне кубера.
- `kubectl get pods -A` покажет, что твои приложения запущены в k8s (`Running`).
- Пробрось порт: `kubectl port-forward svc/frontend-service 3000:80` (или настрой `/etc/hosts` для Ingress, например `127.0.0.1 minionbank.local` и зайди в браузер).

**Кастомные манифесты:**
На этом этапе мы написали "чистые" (сырые) манифесты Kubernetes. На следующем этапе мы превратим их в универсальные шаблоны (Helm), чтобы деплоить сразу в 2 окружения: **dev** и **prod**.

---

## Неделя 4: Шаблонизация с Helm (Dev и Prod окружения)

**Что нужно изучить:**
- Структура Helm-чарта (`Chart.yaml`, `templates/`, `values.yaml`).
- Шаблонизация в Helm (переменные `{{ .Values... }}`, функции, `if/else`).
- Зачем нужен Helm для своих собственных сервисов, а не только для сторонних (удобное управление версиями и конфигами для разных сред).

**Что конкретно сделать в коде:**
1. Создай свой собственный Helm-чарт для бэкенда (и потом для фронтенда):
   ```bash
   helm create minion-backend
   ```
2. Очисти директорию `templates/` от стандартного мусора и перенеси туда свои манифесты `deployment.yaml`, `service.yaml`, `ingress.yaml` с предыдущего этапа.
3. Замени захардкоженные значения (имена образов, реплики, хосты Ingress) на переменные:
   ```yaml
   # Пример куска templates/deployment.yaml
   image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
   replicas: {{ .Values.replicaCount }}
   ```
4. Создай два файла конфигурации: `values-dev.yaml` (например: 1 реплика, домен dev.minionbank.local) и `values-prod.yaml` (3 реплики, домен prod.minionbank.local).

**Как проверить, что это работает:**
- Сделай dry-run (проверку рендера) для dev: `helm template my-backend-dev ./minion-backend -f values-dev.yaml`.
- Установи чарт в кластер в разные namespace (`dev` и `prod`).

---

## Неделя 5: GitOps и ArgoCD

**Что нужно изучить:**
- Парадигма GitOps: конфигурация инфраструктуры и приложений тоже хранится в Git, и *он* является единственным источником правды.
- Архитектура ArgoCD: как ArgoCD постоянно сверяет состояние кластера (Live State) с манифестами в Git (Target State) и автоматически синхронизирует их.

**Что конкретно сделать в коде:**
1. Выдели отдельный репозиторий под инфраструктуру (`minion-bank-infra`), куда ты положишь свой Helm чарт и файлы `values-dev.yaml` / `values-prod.yaml`.
2. Установи ArgoCD в свой k8s-кластер через официальные манифесты.
3. Напиши объект `Application` для ArgoCD, в котором укажешь, что приложение `minion-backend-dev` должно брать код из Git-репозитория `minion-bank-infra` с файлом `values-dev.yaml` и разворачиваться в namespace `dev`.
4. Сделай такой же `Application` для `prod`.

**Как проверить, что это работает:**
- Зайди в Web UI ArgoCD (через port-forward).
- Поменяй версию образа в `values-dev.yaml` в репозитории `minion-bank-infra` и сделай push.
- Убедись, что ArgoCD мгновенно увидел изменения (Out of Sync) и сам обновил поды бэкенда в кластере k8s! Без участия CI пайплайна самого сервиса.

---

## Неделя 6: Безопасное хранение секретов (HashiCorp Vault)

**Что нужно изучить:**
- Концепции GitLab CI: stages, pipelines, Docker-in-Docker / Kaniko (рекомендую Kaniko для безопасной сборки в k8s).
- GitLab Container Registry.
- Обновление k8sDeployment из пайплайна (рассмотри связку с GitLab Agent for Kubernetes или просто запуск `kubectl` внутри пайплайна).

**Что конкретно сделать в коде:**
В каждом из репозиториев (Backend, Frontend) создай `.gitlab-ci.yml`.

*Пример `.gitlab-ci.yml` (Базовая структура):*
```yaml
stages:
  - test
  - build
  - deploy

variables:
  DOCKER_IMAGE: $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA

test_app:
  stage: test
  image: golang:1.22
  script:
    - go test -v ./...

build_image:
  stage: build
  image:
    name: gcr.io/kaniko-project/executor:debug
    entrypoint: [""]
---

## Неделя 7: Умный CI/CD (GitLab CI/CD с выбором деплоя)

**Что нужно изучить:**
- Продвинутые концепции GitLab CI: переменные уровня pipeline, `rules`, `if`.
- Автоматический коммит из одного репозитория (кода бэкенда) в другой (репозиторий инфраструктуры ArgoCD) с использованием Deploy Tokens или SSH ключей.

**Что конкретно сделать в коде:**
В репозитории Backend (и Frontend) создай умный `.gitlab-ci.yml`.
У тебя должен быть выбор, как именно деплоить приложение по окончании сборки Docker-образа. Это решается через переменную CI/CD, например `DEPLOY_METHOD`.

1. **Если `DEPLOY_METHOD == "helm"` (Традиционный Push-деплой):**
   Пайплайн сам скачивает утилиту `helm`, настраивает `kubeconfig` и запускает `helm upgrade --install ...` напрямую в кластер k8s.
2. **Если `DEPLOY_METHOD == "argocd"` (Современный GitOps Pull-деплой):**
   Пайплайн **вообще не трогает k8s**. Вместо этого джоба клонирует infra-репозиторий, меняет тег образа (например через `sed` или `yq`) в файле `values-dev.yaml`, делает `git commit` и `git push`. Дальше за дело берется ArgoCD, как мы настроили на Неделе 5.

**Как проверить, что это работает:**
- Запусти пайплайн ручками с флагом `DEPLOY_METHOD=helm`. В логах посмотришь, как отработал helm.
- Запусти с флагом `DEPLOY_METHOD=argocd`. Посмотри, что в инфра-репу прилетел коммит от бота, и ArgoCD начал раскатку.

---

## Неделя 8: Инструментирование и Метрики (Prometheus & Grafana)

**Что нужно изучить:**
- Библиотека `prometheus/client_golang` для Go.
- Prometheus Operator (`kube-prometheus-stack`).
- Структура кастомных метрик: Counters, Gauges, Histograms.

**Что конкретно сделать в коде:**
1. В Go-коде (Backend) добавь роут `/metrics` и зарегистрируй 2-3 метрики:
   - Общее количество запросов (Counter).
   - Длительность выполнения HTTP запроса или запроса к БД (Histogram / Summary).
2. Разверни Kube Prometheus Stack:
   ```fish
   helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack -n monitoring --create-namespace
   ```
3. Добавь `ServiceMonitor` ресурс в k8s для своего backend Service, чтобы Prometheus "автоматически" начал забирать `/metrics`.
4. Войди в Grafana через port-forward и построй простой дашборд.

**Как проверить, что это работает:**
- Перейди локально по http://localhost/metrics (если пробросил порт) — должен отдаваться сырой текст Prometheus.
- В Grafana построй график: `rate(http_requests_total[5m])` — он должен показывать твои реальные запросы.

---

## Неделя 9: Централизованное логирование (Loki / ELK)

**Что нужно изучить:**
- PLG стек (Promtail, Loki, Grafana) — гораздо легковеснее для локалки чем ELK.
- Structured logging в Go (библиотека `slog` из stdlib или `zap`).

**Что конкретно сделать в коде:**
1. Переведи логирование Backend и Frontend в JSON-формат, чтобы парсеру было проще.
2. Установи Loki и Promtail через Helm:
   ```fish
   helm repo add grafana https://grafana.github.io/helm-charts
   helm install loki grafana/loki-stack --set promtail.enabled=true,grafana.enabled=false -n monitoring
   ```
   (Grafana у тебя уже есть с прошлой недели).
3. Добавь Loki DataSource в уже существующую Grafana.

**Как проверить, что это работает:**
- Открой Grafana в браузере. Перейди в раздел Explore.
- Выбери Loki как источник данных.
- Выполни запрос: `{app="backend"}`. Ты должен увидеть поток логов своего Go-приложения в реальном времени. Можно отфильтровать ошибки: `{app="backend"} |= "error"`.
