# Этап 7: Умный CI/CD (GitLab CI/CD с выбором деплоя)

В Этапе 5 мы вручную выполняли `helm upgrade`. А в Этапе 6 внедрили `ArgoCD`, который сам стягивает изменения по коммиту.
Теперь нам нужно настроить пайплайн GitLab так, чтобы у нас был выбор: деплоить напрямую через Helm (Push) ИЛИ изменить конфигурацию в Git-репозитории инфраструктуры (Pull через ArgoCD).

## 1. Теория: Что нужно выучить

### Переменные уровня Pipeline (CI/CD Variables)
В настройках проекта (или при ручном запуске пайплайна) мы можем передать переменную, например `DEPLOY_METHOD`. Опираясь на нее, GitLab решит, какую "джобу" запускать: с `helm` или с `git commit`.

### Что должен сделать пайплайн при ArgoCD (GitOps)?
Пайплайн исходного кода (допустим, `backend`) собирает новый Docker-образ с уникальным тегом (обычно это `$CI_COMMIT_SHORT_SHA`). 
Сам ArgoCD ничего не знает об успешной сборке. Поэтому задача пайплайна:
1. Склонировать репозиторий `minion-bank-infra`.
2. Найти файл `dev-env/backend-values.yaml`.
3. Заменить там строчку `tag: "старый_хэш"` на `tag: "новый_хэш"`.
4. Сделать коммит и запушить обратно в `minion-bank-infra`. 
*(ArgoCD заметит новый коммит и сам обновит поды).*

---

## 2. Практика: Что конкретно сделать в коде

### 2.1 Подготовка
Тебе понадобится **Deploy Token** (или Access Token с правами на запись) от инфраструктурного репозитория `minion-bank-infra`, чтобы скрипт из пайплайна `backend`-а мог туда пушить.
1. Зайди в настройки репозитория `minion-bank-infra` -> Settings -> Access Tokens. Выпусти токен с правами `write_repository`.
2. Скопируй токен.
3. Зайди в настройки репозитория `backend` -> Settings -> CI/CD -> Variables.
4. Добавь переменную `INFRA_REPO_TOKEN` и вставь туда этот токен. Сделай её Masked (чтобы не светилась в логах).

### 2.2 Умный `.gitlab-ci.yml`
Создай/обнови файл `.gitlab-ci.yml` в корне репозитория `backend`:

```yaml
stages:
  - build
  - deploy

variables:
  DOCKER_IMAGE: $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA
  # По умолчанию деплоим через Argo (переменную можно переопределить при ручном запуске)
  DEPLOY_METHOD: "argocd"

# --------------- BUILD ---------------
build-and-push:
  stage: build
  image:
    name: gcr.io/kaniko-project/executor:v1.14.0-debug
    entrypoint: [""]
  script:
    - echo "{\"auths\":{\"$CI_REGISTRY\":{\"username\":\"$CI_REGISTRY_USER\",\"password\":\"$CI_REGISTRY_PASSWORD\"}}}" > /kaniko/.docker/config.json
    - /kaniko/executor 
      --context $CI_PROJECT_DIR 
      --dockerfile $CI_PROJECT_DIR/Dockerfile 
      --destination $DOCKER_IMAGE
  only:
    - main

# --------------- DEPLOY TIER: HELM ---------------
deploy-helm:
  stage: deploy
  image: alpine/helm:3.12.0
  rules:
    # Запускаем эту джобу ТОЛЬКО если переменная равна helm
    - if: '$DEPLOY_METHOD == "helm"'
  script:
    - echo "$KUBECONFIG_CONTENT" > /tmp/kubeconfig
    - export KUBECONFIG=/tmp/kubeconfig
    - helm upgrade --install backend-app ./minion-backend-chart -n dev \
        --set image.repository=$CI_REGISTRY_IMAGE \
        --set image.tag=$CI_COMMIT_SHORT_SHA

# --------------- DEPLOY TIER: ARGOCD (GITOPS) ---------------
deploy-argocd:
  stage: deploy
  image: alpine/git:v2.40.1
  rules:
    # Запускаем эту джобу ТОЛЬКО если переменная равна argocd
    - if: '$DEPLOY_METHOD == "argocd"'
  script:
    # Настраиваем Git
    - git config --global user.email "ci-bot@minionbank.local"
    - git config --global user.name "GitLab CI Bot"
    
    # Клонируем инфраструктурный репозиторий, используя токен для авторизации
    # Обрати внимание, что мы подставляем токен в URL
    - git clone https://oauth2:${INFRA_REPO_TOKEN}@github.com/rusya148/minion-bank-infra.git /tmp/infra-repo
    - cd /tmp/infra-repo
    
    # Меняем тег образа с помощью потокового редактора sed. 
    # Это найдет строчку "tag: что_угодно" и заменит на "tag: НАШ_НОВЫЙ_ХЭШ"
    - sed -i "s/tag:.*/tag: \"${CI_COMMIT_SHORT_SHA}\"/" dev-env/backend-values.yaml
    
    # Коммитим и отправляем
    - git add dev-env/backend-values.yaml
    - git commit -m "Auto-update backend image tag to ${CI_COMMIT_SHORT_SHA}"
    - git push origin main
```

---

## 3. Как проверить, что это работает

1. **Обычный пуш (ArgoCD):**
   - Сделай любой коммит в репозиторий `backend` и запушь в `main`.
   - Pipeline запустится. Stage `deploy` выберет джобу `deploy-argocd` (так как дефолтная переменная `argocd`).
   - Зайди в репозиторий `minion-bank-infra`, ты должен увидеть свежий коммит от пользователя `GitLab CI Bot` измененным файлом `backend-values.yaml`.
   - Зайди в UI ArgoCD и убедись, что поды обновились на новый образ.

2. **Запуск через Helm (Ручной Pipeline):**
   - В интерфейсе GitLab твоего `backend` перейди в **CI/CD -> Pipelines**.
   - Нажми синюю кнопку **Run pipeline** в правом верхнем углу.
   - Выбери ветку `main`.
   - В разделе **Variables** введи:
     - Key: `DEPLOY_METHOD`
     - Value: `helm`
   - Нажми `Run pipeline`.
   - Теперь Stage `deploy` проигнорирует Git-джоюу и запустит `deploy-helm`, сделав деплой напрямую в кластер! (убедись, что у тебя настроена переменная `KUBECONFIG_CONTENT`).

**Выбор деплоя готов! В реальных продакшенах часто используют исключительно ArgoCD (Pull), но Helm-деплой (Push) полезен для быстрых экспериментов или эфемерных сред.**
