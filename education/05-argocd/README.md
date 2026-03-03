# Этап 5: GitOps и ArgoCD

На прошлом этапе мы научились шаблонизировать наши k8s манифесты с помощью Helm для разных окружений (dev и prod). Но у нас осталась проблема: чтобы их применить, кто-то (ты или пайплайн GitLab) должен выполнить команду `helm upgrade` с доступом к кластеру. Это называется **Push-деплой**.

Такой подход требует хранить админские доступы от кластера в CI/CD системе. **GitOps** решает эту проблему, переворачивая логику (переходя к **Pull-деплою**).

## 1. Теория: Что нужно выучить

### Что такое GitOps?
Это концепция, при которой **Git репозиторий является единственным источником правды** для твоей инфраструктуры. Если ты хочешь изменить количество реплик с 2 на 3, ты не делаешь `kubectl scale`. Ты делаешь git-коммит в репозиторий, меняя цифру в `values-prod.yaml`, и инфраструктура *сама* приводит себя в соответствие с кодом в Git.

### Зачем нужен ArgoCD?
ArgoCD — это оператор, который устанавливается *внутрь* твоего K8s кластера. Его задача:
1. Раз в 3 минуты заглядывать в указанный тобой Git-репозиторий (например `minion-bank-infra`).
2. Сравнивать файл манифестов из Git с тем, что сейчас реально запущено в кластере (Live State).
3. Если есть разница (например, в git новая версия image) — у приложения появляется статус `Out of Sync`.
4. ArgoCD автоматически (или по кнопке) делает `Sync`, приводя кластер в соответствие с Git.

---

## 2. Практика: Что конкретно сделать в коде

### 2.1 Подготовка инфраструктурного репозитория
Тебе нужен отдельный репозиторий, где хранится только конфигурация. Не код на Go или React!
Создай Git-репозиторий `minion-bank-infra`.
Положи внутрь папку со своим Helm-чартом бэкенда из прошлого урока:

```text
minion-bank-infra/
├── minion-backend/      # Собственно сам чарт
│   ├── Chart.yaml
│   └── templates/
├── dev-env/
│   └── backend-values.yaml  # Твои values для dev окружения
└── prod-env/
    └── backend-values.yaml  # Твои values для prod окружения
```
Сделай `git commit` и `git push` в удаленный репозиторий (GitLab/GitHub).

### 2.2 Установка ArgoCD в кластер
Запусти это в терминале `bash`:

```bash
# Создаем namespace для ArgoCD
kubectl create namespace argocd

# Устанавливаем официальные манифесты ArgoCD
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Ждем пока поды поднимутся
kubectl wait --namespace argocd --for=condition=ready pod --selector=app.kubernetes.io/name=argocd-server --timeout=90s
```

### 2.3 Доступ к интерфейсу ArgoCD
По умолчанию ArgoCD сервер не смотрит наружу. Чтобы в него зайти:
1. Пробрось порт:
   ```bash
   kubectl port-forward svc/argocd-server -n argocd 8080:443
   ```
2. Открой `https://localhost:8080` (именно HTTPS, проигнорируй предупреждение сертификата).
3. Дефолтный логин: `admin`.
4. Пароль генерируется при установке. Достань его командой:
   ```bash
   kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo
   ```

### 2.4 Настройка приложения (ArgoCD Application)

Теперь нам нужно сказать ArgoCD, за каким репозиторием следить. Пишем специальный K8s манифест типа `Application` (его можно создать прямо из Web UI, но мы сделаем как профи — кодом).

Создай файл `argocd-backend-dev.yaml` (можешь положить его тоже в infra репу):
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: minion-backend-dev
  namespace: argocd
spec:
  project: default
  source:
    # URL твоего инфраструктурного репозитория! (если он приватный, в UI ArgoCD надо будет добавить SSH ключ)
    repoURL: 'https://github.com/rusya148/minion-bank-infra.git' 
    # В какой папке лежит Chart
    path: minion-backend
    targetRevision: HEAD
    helm:
      # Указываем, какие value файлы наложить поверх стандартных
      valueFiles:
      - ../dev-env/backend-values.yaml
  destination:
    server: 'https://kubernetes.default.svc' # Деплоим в тот же кластер, где стоит сам argo
    namespace: dev # В какой namespace деплоить
  syncPolicy:
    automated: # Автоматическая синхронизация при любом пуше в Git
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true # Создаст namespace dev, если его нет
```

Примени объект:
```bash
kubectl apply -f argocd-backend-dev.yaml
```

---

## 3. Как проверить, что это работает

1. **Зайди в Web UI ArgoCD:**
   На главной странице ты увидишь приложение `minion-backend-dev`. Оно должно стать зеленым (Synced / Healthy).
2. **Проверь кластер:**
   ```bash
   kubectl get pods -n dev
   ```
   Ты увидишь поды бэкенда. ArgoCD сам отрендерил твой Helm Chart с `backend-values.yaml` и задеплоил в K8s!

3. **Магия GitOps:**
   - Открой репозиторий `minion-bank-infra` в редакторе (локально).
   - Измени файл `dev-env/backend-values.yaml` (например, поставь `replicaCount: 3`).
   - Сделай `git commit -m "scale up to 3 replicas"` и `git push`.
   - Не трогай `kubectl` и `helm`! Просто перейди в браузер в окно ArgoCD и подожди пару минут (или нажми `Refresh`).
   - Ты увидишь, как статус станет `OutOfSync`, ArgoCD запустить синхронизацию (желтая крутилка), и через секунду в кластере появятся новые поды.

**Поздравляю, ты внедрил GitOps! Теперь твоему CI-пайплайну (GitLab) больше не нужно ходить в кластер. Теперь задача пайплайна — просто собрать Docker-образ и сгенерировать коммит (изменить хэш образа в values) в репозиторий `minion-bank-infra`. Об этом мы поговорим в Этапе 7!**
