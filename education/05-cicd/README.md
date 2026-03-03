# Этап 5: CI/CD автоматизация (GitLab CI/CD)

До этого мы собирали и деплоили наши образы (Frontend / Backend) руками и применяли манифесты Kubernetes через терминал. Это долго, чревато ошибками и не масштабируется для команды.
Теперь наша цель — настроить GitLab CI/CD, чтобы каждый push в ветку `main` автоматически сканировал код, запускал тесты, собирал Docker-образ и выкатывал его в наш кластер.

## 1. Теория: Что нужно выучить

### Что такое GitLab CI/CD?
Это встроенная в GitLab система непрерывной интеграции (Continuous Integration - сборка и тестирование кода) и непрерывного развертывания (Continuous Deployment - деплой в k8s/сервера). Вся конфигурация описывается в корне репозитория в файле `.gitlab-ci.yml`.

### Архитектура выполнения (GitLab Runner)
Сам веб-интерфейс GitLab ничего не собирает. Этим занимаются **Runners** — агенты, которые подключаются к GitLab-серверу, забирают задачи (Jobs) из очереди и выполняют их (обычно внутри эфемерных Docker-контейнеров).
В случае проектов `Minion Bank`, так как они лежат на `gitlab.com`, мы можем использовать их бесплатные "Shared Runners".

### Структура `.gitlab-ci.yml`
1. **Stages:** Этапы пайплайна, которые выполняются строго последовательно (например, `test` -> `build` -> `deploy`). Если этап `test` упал с ошибкой, `build` не запустится.
2. **Jobs:** Конкретная задача внутри этапа (например, `unit-tests`, `lint-code` — могут работать параллельно внутри одного этапа).
3. **Image:** Базовый Docker-контейнер, в котором запустится джоба (например `golang:1.22` для билда бэкенда).
4. **Script:** Набор shell/bash команд, которые нужно выполнить в рамках Job.
5. **Variables:** Переменные пайплайна. Некоторые из них (тот же пароль от реджистри Docker) мы можем спрятать в настройках репозитория (Settings -> CI/CD -> Variables), чтобы не "светить" в коде.

### Что такое Kaniko?
Обычно, чтобы собрать Docker-образ (команда `docker build`), тебе нужен работающий демон Docker (`dockerd`). Но запустить Docker *внутри* докера (Docker-in-Docker или DinD) в кластерах считается небезопасным и сложным делом.
**Kaniko** от Google решает эту проблему. Это специальный инструмент, который умеет собирать Docker-образы "без докера", читая слои напрямую из `Dockerfile`. Мы будем использовать Kaniko для этапа `build`.

---

## 2. Практика: Что конкретно сделать в коде

Давай напишем `.gitlab-ci.yml` для репозитория **Backend (Go)**. *(Для фронтенда на React логика будет идентична, только вместо `golang:1.22` будет `node:20`, а вместо `go test` — `npm test` / `npm run build`)*.

### 2.1 Подготовка GitLab Container Registry
Каждый репозиторий на GitLab имеет свой собственный Container Registry (аналог Docker Hub), куда мы можем публиковать наши `rusya148/backend` образы.
Нам не нужно настраивать логины-пароли для него в CI, потому что GitLab автоматически предоставляет временные токены в переменных `$CI_REGISTRY`, `$CI_REGISTRY_USER` и `$CI_REGISTRY_PASSWORD` во время выполнения пайплайна.

### 2.2 Пишем пайплайн

Создай файл `.gitlab-ci.yml` в корне репозитория backend:

```yaml
stages:
  - lint
  - test
  - build
  - deploy

# Переменная с будущим именем образа (используем Registry проекта и тегируем хэшом коммита)
variables:
  DOCKER_IMAGE: $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA

# --------------- LINT & TEST ---------------
golangci-lint:
  stage: lint
  image: golangci/golangci-lint:v1.55.2
  script:
    - golangci-lint run -v

unit-tests:
  stage: test
  image: golang:1.22
  script:
    - go test -v -cover ./...

# --------------- BUILD ---------------
build-and-push:
  stage: build
  # Используем Kaniko вместо обычного docker:dind
  image:
    name: gcr.io/kaniko-project/executor:v1.14.0-debug
    entrypoint: [""]
  script:
    # Конфигурируем доступ Kaniko к GitLab Registry с помощью встроенных токенов
    - echo "{\"auths\":{\"$CI_REGISTRY\":{\"username\":\"$CI_REGISTRY_USER\",\"password\":\"$CI_REGISTRY_PASSWORD\"}}}" > /kaniko/.docker/config.json
    
    # Запускаем сборку
    # --context = папка с кодом
    # --destination = куда пушить готовый образ
    - /kaniko/executor
      --context $CI_PROJECT_DIR
      --dockerfile $CI_PROJECT_DIR/Dockerfile
      --destination $DOCKER_IMAGE
  # Запускать билд только если коммит был в ветку main
  only:
    - main

# --------------- DEPLOY ---------------
deploy-to-k8s:
  stage: deploy
  image: bitnami/kubectl:latest
  script:
    # В настройках проекта GitLab нужно заранее добавить переменную KUBECONFIG_CONTENT
    # с содержимым твоего локального kubeconfig'а (~/.kube/config), 
    # чтобы GitLab Runner мог достучаться до кластера.
    # (*Примечание: если твой кластер строго локальный и не торчит наружу, 
    # то придется использовать GitLab Runner установленный локально на твою машину, а не Shared от gitlab.com)
    - echo "$KUBECONFIG_CONTENT" > /tmp/kubeconfig
    - export KUBECONFIG=/tmp/kubeconfig
    
    # Если кластер видит команду — обновляем образ в деплойменте!
    - kubectl set image deployment/backend-app backend=$DOCKER_IMAGE --record
    - kubectl rollout status deployment/backend-app
  only:
    - main
```

*(Заметка об инфраструктуре: Если твой кластер запущен на `localhost` или в закрытой домашней подсети через Kind, публичные раннеры GitLab (Shared Runners) до него не "достучатся" на стадии `deploy-to-k8s`. Тебе нужно будет либо выставить API K8s наружу через ngrok/port-forwarding, либо — установить свой приватный gitlab-runner на ту же Ubuntu, где крутится кластер, и зарегистрировать его в проекте. Это хорошая практика!)*

---

## 3. Как проверить, что это работает

1. **Закоммить `.gitlab-ci.yml` и push в репозиторий.**
   ```bash
   git add .gitlab-ci.yml
   git commit -m "Add CI/CD pipeline"
   git push origin main
   ```
2. Открой интерфейс GitLab проекта, перейди в меню: **Build -> Pipelines**.
3. Ты должен увидеть, как поехали Stages: `lint` (если линтер прошел) -> `test` -> `build` -> `deploy`.
4. Нажми на джобу `build-and-push`. В логах (терминале) должно быть видно скачивание слоев и пуш образа.
5. После успешного билда зайди в меню **Deploy -> Container Registry**. Убедись, что твой образ `rusya148/backend:<хэш_коммита>` появился в списке.
6. Выполни в кластере `kubectl get pods -w` во время фазы `deploy`. Ты увидишь, как K8s плавно "гасит" старые поды Backend-а и поднимает новые с только что собранным хэшом образа.

Поздравляю, теперь твой код доезжает от `git push` до продакшена полностью автоматически! Мы переходим к финальному этапу: Мониторинг и Логи (Prometheus + Grafana + Loki).
