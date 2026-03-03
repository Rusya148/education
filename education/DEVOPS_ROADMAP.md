# DevOps Roadmap: Minion Bank (Go + React)

**Роль:** Senior DevOps Engineer / Инфраструктурный архитектор
**Стек:** Go, React, PostgreSQL, Kafka, Terraform, Ansible, GitLab CI, Kubernetes, Vault, Prometheus, Grafana, Loki.
**Окружение:** Arch Linux, fish shell, i3 wm.

Этот план разбит на 6 логических этапов (недель), двигаясь от базовой инфраструктуры до продвинутой наблюдаемости и безопасности.

---

## Неделя 1: Infrastructure as Code (Terraform) и базовая инфраструктура

**Что нужно изучить:**
- Основы Terraform (состояния, провайдеры, модули).
- Использование локальных k8s-кластеров через `kind` (Kubernetes in Docker) или `k3d`.
- Базовый Ansible (в контексте первоначальной подготовки ОС, если когда-то будешь деплоить на "голое железо" / VPS, хотя для локалки на Arch Linux нам достаточно поставить пакеты).

**Что конкретно сделать в коде:**
1. **Подготовка Arch Linux:**
   Установи необходимые зависимости через твой привычный пакетный менеджер из под `fish`:
   ```fish
   sudo pacman -S docker terraform ansible kubectl helm k9s
   yay -S kind # или k3d-bin
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
- Stateful приложения в k8s: `StatefulSet`, `PV`, `PVC`.
- Работа с пакетным менеджером Helm.

**Что конкретно сделать в коде:**
1. Разверни NGINX Ingress Controller для обработки входящего трафика внутрь кластера.
2. Подними **PostgreSQL** и **Kafka** в K8s с помощью `Helm`. Запиши параметры в кастомные `values.yaml` (`kubernetes/values/postgres-values.yaml`, и т.д.).
   ```fish
   helm repo add bitnami https://charts.bitnami.com/bitnami
   helm install postgres bitnami/postgresql -f kubernetes/values/postgres-values.yaml
   ```
3. Напиши манифесты для своих приложений:
   - `kubernetes/backend/`: `deployment.yaml`, `service.yaml`, `ingress.yaml`.
   - `kubernetes/frontend/`: `deployment.yaml`, `service.yaml`, `ingress.yaml` (или настрой Nginx Ingress отдавать статику).

**Как проверить, что это работает:**
- `kubectl get pods -A` покажет, что БД, Kafka и твои приложения запущены (`Running`).
- Пробрось порт: `kubectl port-forward svc/frontend-service 3000:80` (или настрой `/etc/hosts` для Ingress, например `127.0.0.1 minionbank.local` и зайди в браузер).

---

## Неделя 3: CI/CD автоматизация (GitLab CI/CD)

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
  script:
    - mkdir -p /kaniko/.docker
    - echo "{\"auths\":{\"$CI_REGISTRY\":{\"username\":\"$CI_REGISTRY_USER\",\"password\":\"$CI_REGISTRY_PASSWORD\"}}}" > /kaniko/.docker/config.json
    - /kaniko/executor --context $CI_PROJECT_DIR --dockerfile $CI_PROJECT_DIR/Dockerfile --destination $DOCKER_IMAGE

deploy_k8s:
  stage: deploy
  image: bitnami/kubectl:latest
  script:
    # Требуется настройка kubeconfig через переменные CI/CD
    - kubectl set image deployment/backend backend=$DOCKER_IMAGE
    - kubectl rollout status deployment/backend
```

**Как проверить, что это работает:**
- Сделай git push. В интерфейсе GitLab в разделе `CI/CD -> Pipelines` ползунок дойдет до зеленой галочки.
- В GitLab `Container Registry` соберется Docker-образ твоего бэкенда.
- В кластере автоматически пересоздадутся поды с новым образом (можно отследить через `k9s`).

---

## Неделя 4: Безопасное хранение секретов (HashiCorp Vault)

**Что нужно изучить:**
- Архитектура HashiCorp Vault.
- Механизм авторизации k8s (Kubernetes Auth Method) в Vault.
- Vault Agent Sidecar Injector (как автоматически прокидывать секреты в поды, чтобы приложение читало их как файл или переменные окружения, а разработчики не видели их в открытом виде).

**Что конкретно сделать в коде:**
1. Разверни Vault в dev-режиме локально через Helm.
2. Настрой Kubernetes Auth:
   ```fish
   vault auth enable kubernetes
   vault write auth/kubernetes/config kubernetes_host="https://kubernetes.default.svc:443"
   ```
3. Создай секрет с кредами от PostgreSQL в Vault (`secret/data/backend-db`).
4. Добавь аннотации в `kubernetes/backend/deployment.yaml`:
   ```yaml
   template:
     metadata:
       annotations:
         vault.hashicorp.com/agent-inject: "true"
         vault.hashicorp.com/role: "backend-role"
         vault.hashicorp.com/agent-inject-secret-db-config: "secret/data/backend-db"
   ```
5. В Go-бэкенде настрой чтение файла `/vault/secrets/db-config` или парсинг переменных окружения, которые Vault сгенерирует.

**Как проверить, что это работает:**
- Удали хардкод или обычный k8s Secret с паролем от БД. `kubectl delete secret db-pass`.
- Рестартни под бэкенда. Сделай `kubectl exec -it <backend-pod_name> -c backend -- fish` (или sh).
- Проверь что файл есть: `cat /vault/secrets/db-config`.
- По логам Backend-а убедись, что он успешно подключился к PostgreSQL.

---

## Неделя 5: Инструментирование и Метрики (Prometheus & Grafana)

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

## Неделя 6: Централизованное логирование (Loki / ELK)

**Что нужно изучить:**
- PLG стек (Promtail, Loki, Grafana) — гораздо легковеснее для Arch Linux/локалки чем ELK.
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
