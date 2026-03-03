# Этап 4: Безопасное хранение секретов (HashiCorp Vault)

На предыдущем шаге мы передавали пароли от PostgreSQL как простые переменные окружения прямо в манифест k8s (`backend-deployment.yaml`). Это огромная дыра в безопасности: любой, кто имеет доступ к репозиторию или права на просмотр деплоймента в кластере, увидит пароль.

Чтобы этого избежать, мы внедряем **HashiCorp Vault**.

## 1. Теория: Что нужно выучить

### Что такое HashiCorp Vault?
Vault — это инструмент для безопасного хранения и управления секретами (пароли, API-ключи, сертификаты).
Вместо того чтобы хардкодить секреты, приложение обращается к Vault по API (или читает файл, который Vault сам подложил), забирает нужный пароль и использует его в памяти.

### Как Vault интегрируется с Kubernetes?
Мы будем использовать механизм авто-инъекции (Auto-injection) через Sidecar-патерн.
1. Vault запускает специальный сервис (Webhook) внутри твоего K8s кластера.
2. Когда ты деплоишь под своего бэкенда, этот Webhook перехватывает запрос и смотрит на Аннотации (Annotations).
3. Если он видит аннотацию `vault.hashicorp.com/agent-inject: "true"`, он автоматически подселяет к твоему Go-контейнеру ещё один маленький контейнер — `vault-agent`.
4. `vault-agent` авторизуется в Vault под специальной сервисной учеткой (ServiceAccount) твоего пода, скачивает нужный секрет (например, данные от БД) и сохраняет его во временный файл в общей памяти (RAM-disks), например в `/vault/secrets/db-config`.
5. Твоему Go-приложению остается просто прочитать этот локальный файл!

---

## 2. Практика: Что конкретно сделать в коде

### 2.1 Установка Vault в Kubernetes
Добавляем репозиторий HashiCorp и ставим Vault через Helm в dev-режиме:
```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm install vault hashicorp/vault --set "server.dev.enabled=true" --set "injector.enabled=true"
```
*Dev-режим значит, что Vault поднимется мгновенно, в оперативной памяти, с рутовым токеном `root`, и без сложной настройки распечатывания (unsealing). Идеально для обучения.*

### 2.2 Включаем Kubernetes Auth в Vault
Открываем интерактивный терминал (bash) внутри пода Vault:
```bash
kubectl exec -it vault-0 -- /bin/sh
```

Внутри Vault-сервера:
Включаем метод авторизации через k8s:
```bash
vault auth enable kubernetes
```
Указываем Vault-у, где находится API сервер k8s (чтобы он мог проверять токены подов):
```bash
vault write auth/kubernetes/config \
    kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443"
```

### 2.3 Создаем секрет и политику доступа (Role)
Оставаясь внутри пода `vault-0`, добавим пароль от нашей БД PostgreSQL в хранилище Vault:

```bash
# Кладем JSON по пути secret/minion-bank/database
vault kv put secret/minion-bank/database password="supersecretdb_pass" username="minion_user" host="postgres-postgresql.default.svc.cluster.local" dbname="minion_bank"
```

Создадим так называемую Политику (Policy), которая разрешает чтение только этого секрета:
```bash
vault policy write backend-policy - <<EOF
path "secret/data/minion-bank/database" {
  capabilities = ["read"]
}
EOF
```

Свяжем эту политику с ServiceAccount-ом нашего бэкенда (назовем роль `backend-role`):
```bash
vault write auth/kubernetes/role/backend-role \
    bound_service_account_names=backend-sa \
    bound_service_account_namespaces=default \
    policies=backend-policy \
    ttl=24h
exit # Выходим из пода vault-0
```

### 2.4 Обновляем Deployment твоего Go-приложения (Инъекция секрета)

Теперь нам нужно обновить манифесты, чтобы использовать этот секрет.
Сначала создадим ServiceAccount (`backend-sa`):
В файл `kubernetes/backend-sa.yaml` добавь:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: backend-sa
  namespace: default
```
Примени: `kubectl apply -f kubernetes/backend-sa.yaml`

Затем обнови свой `kubernetes/backend-deployment.yaml`.
Удали блок `env` с хардкодом паролей, и добавь `serviceAccountName` и аннотации (annotations) для Vault!

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend-app
  labels:
    app: backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
      annotations:
        # Включаем инъекцию
        vault.hashicorp.com/agent-inject: 'true'
        # Указываем, под какой ролью стучаться в Vault
        vault.hashicorp.com/role: 'backend-role'
        # Говорим, из какого пути в Vault забрать секрет и в какой файл положить (db-creds)
        vault.hashicorp.com/agent-inject-secret-db-creds: 'secret/data/minion-bank/database'
        # Шаблонизируем файл db-creds (например, в формате .env или export)
        vault.hashicorp.com/agent-inject-template-db-creds: |
          {{- with secret "secret/data/minion-bank/database" -}}
          DB_HOST={{ .Data.data.host }}
          DB_USER={{ .Data.data.username }}
          DB_PASSWORD={{ .Data.data.password }}
          DB_NAME={{ .Data.data.dbname }}
          {{- end -}}
    spec:
      serviceAccountName: backend-sa # Важно! Именно для этого аккаунта мы настраивали права в Vault
      containers:
      - name: backend
        image: rusya148/minion-backend:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
        # В случае с Go (или любым другим) можно заставить приложение читать файл `/vault/secrets/db-creds` 
        # (например использовать godotenv для загрузки переменных из этого файла)
```

Примени обновленный Deployment:
```bash
kubectl apply -f kubernetes/backend-deployment.yaml
```

---

## 3. Как проверить, что это работает

1. **Проверь поды бэкенда:**
   ```bash
   kubectl get pods
   ```
   Ты должен заметить, что у подов `backend-app` теперь `2/2` контейнера (`READY`) вместо `1/1`. Второй контейнер — это тот самый `vault-agent`.

2. **Зайди внутрь пода Go и проверь файл с секретами:**
   ```bash
   kubectl exec -it <имя-пода-backend-app> -c backend -- sh
   ```
   *Заметь флаг `-c backend`. Мы явно указываем, что хотим зайти в контейнер с Go, а не в vault-agent.*
   
   Выполни внутри:
   ```bash
   cat /vault/secrets/db-creds
   ```
   Если ты видишь там:
   ```text
   DB_HOST=postgres-postgresql.default.svc.cluster.local
   DB_USER=minion_user
   DB_PASSWORD=supersecretdb_pass
   DB_NAME=minion_bank
   ```
   Значит, инъекция работает идеально! Твой бэкенд может читать этот файл и подключаться к базе с абсолютной безопасностью.
