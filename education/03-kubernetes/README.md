# Этап 3: Kubernetes (Оркестрация)

Кластер (через Terraform и Kind) поднят. Теперь нам нужно научиться разворачивать в нём приложения (Frontend, Backend, БД, Брокер сообщений).

## 1. Теория: Что нужно выучить

### Что такое Kubernetes (K8s)?
Kubernetes — это система оркестрации контейнеров. Если Docker просто запускает 1 контейнер, то K8s решает проблемы масштабирования (как запустить 100 контейнеров), рестарта при падении (самовосстановление), балансировки нагрузки между ними и распределения их по разным серверам (нодам).

### Базовые объекты (Ресурсы) K8s:
1. **Pod:** Минимальная единица в k8s. Это "обертка" над одним или несколькими контейнерами (например, твой Go-backend + Sidecar Vault-агента) разделяющими общий IP.
2. **Deployment:** Описывает желаемое состояние подов (например: "Хочу, чтобы всегда работало ровно 3 реплики пода Backend-а"). Если один под умирает — Deployment мгновенно создает новый.
3. **Service:** Поскольку поды смертны и их IP-адреса постоянно меняются, *Service* дает стабильный внутренний IP и DNS-имя (например, `backend-svc`) для балансировки трафика на эти поды внутри кластера.
4. **Ingress / Ingress Controller:** Точка входа в кластер из внешнего мира (интернета). Это "умный" Nginx/HAProxy, который маршрутизирует трафик по URL и доменам (например: запрос на `api.minionbank.local` направить в сервис бэкенда, а `/` — во фронтенд).
5. **ConfigMap & Secret:** Хранилища конфигурационных файлов и секретов. Приложение может примонтировать их как файлы или получить в виде переменных окружения.
6. **StatefulSet & PV/PVC:** Объекты для stateful-приложений, хотя в рамках этого туториала мы вынесем сложные базы данных из Kubernetes в отдельный `docker-compose`.

### Что такое Helm?
Helm — это пакетный менеджер (как `apt` для Ubuntu). Он шаблонизирует манифесты k8s, чтобы можно было установить сложную базу данных вроде PostgreSQL одной командой, просто указав нужный пароль в файле `values.yaml`. 

---

## 2. Практика: Что конкретно сделать в коде

Наша архитектура: Nginx Ingress Controller -> React Frontend UI -> NodePort/Ingress -> Go Backend API -> (PostgreSQL & Kafka).

### 2.1 Установка Ingress Controller
Kind по умолчанию не имеет встроенного Ingress, поэтому нужно его поставить. Это официальный контроллер.
Из терминала `bash` выполни:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# Подожди, пока поды не перейдут в статус Running (может занять минуту)
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

### 2.2 Развертывание PostgreSQL и Kafka (вне Kubernetes)
Так как базы данных и брокеры сообщений сложны в управлении внутри K8s для новичков, мы вынесем их наружу в обычный `docker-compose.yml` в корне проекта на хост-машине:

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: minion_user
      POSTGRES_PASSWORD: secretpassword
      POSTGRES_DB: minion_bank
    ports:
      - "5432:5432"

  kafka:
    image: bitnami/kafka:latest
    environment:
      KAFKA_CFG_NODE_ID: 1
      KAFKA_CFG_PROCESS_ROLES: broker,controller
      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: 1@localhost:9093
      KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER
    ports:
      - "9092:9092"
```

Запусти их на своей Ubuntu:
```bash
docker-compose up -d
```

### 2.3 Написание манифестов для Backend (Go)
Создай файл `kubernetes/backend-deployment.yaml`.
*Предполагается, что образ `rusya148/minion-backend:latest` (или подобный) где-то собран и запушен на Docker Hub или GitLab Registry.*

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend-app
  labels:
    app: backend
spec:
  replicas: 2 # Поднимем 2 экземпляра бэкенда для отказоустойчивости
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
      - name: backend
        image: rusya148/minion-backend:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
        env:
        # Пока что мы передаем креды так. На следующем этапе (Vault) мы это удалим!
        - name: DB_HOST
          # Замени на IP-адрес своей Ubuntu машины в локальной сети, например 192.168.1.10. 
          # Если использовать Docker Desktop, можно писать host.docker.internal, но для linux используй публичный IP шлюза докера или локальной сети.
          value: "192.168.1.X"
        - name: DB_USER
          value: "minion_user"
        - name: DB_PASSWORD
          value: "secretpassword"
        - name: DB_NAME
          value: "minion_bank"
```

Создай `kubernetes/backend-service.yaml`:
```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: backend-svc
spec:
  selector:
    app: backend
  ports:
    - protocol: TCP
      port: 80       # Порт сервиса
      targetPort: 8080 # Порт контейнера внутри пода
```

### 2.4 Настройка маршрутизации (Ingress)
Создай `kubernetes/ingress.yaml`, который скажет Ingress контроллеру: все запросы на `/api` отправлять в бэкенд.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: minion-bank-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: minionbank.local
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: backend-svc
            port:
              number: 80
```

Примени манифесты твоих приложений к кластеру:
```bash
kubectl apply -f kubernetes/backend-deployment.yaml
kubectl apply -f kubernetes/backend-service.yaml
kubectl apply -f kubernetes/ingress.yaml
```

---

## 3. Как проверить, что это работает

1. **Проверь статусы Объектов:**
   ```bash
   kubectl get pods
   kubectl get svc
   kubectl get ingress
   ```
   Ты должен увидеть поды своих приложений, которые должны быть в статусе `Running`. БД и Kafka живут вне кластера, поэтому здесь их не будет.
   
2. **Проверь логи твоего сервиса:**
   Найди точное имя пода через команду выше и глянь логи:
   ```bash
   kubectl logs backend-app-<уникальный хеш>
   ```
   Убедись, что Go-приложение не крашится из-за отсутствия коннекта к БД или Кафке.

3. **Проверь доступ извне (через Ingress):**
   Так как Ingress настроен на домен `minionbank.local`, добавь его в свой файл hosts в Ubuntu:
   ```bash
   sudo echo "127.0.0.1 minionbank.local" >> /etc/hosts
   ```
   
   Сделай запрос в терминале:
   ```bash
   curl http://minionbank.local/api/health
   # Или какой у тебя роут для healthcheck-а в Go. Пайплайн Ingress-а должен его успешно вернуть!
   ```

**(Точно такие же манифесты Deployment, Service, Ingress нужны будут и для React Frontend-а, только там портом чаще всего будет 80, так как фронт собирается как статика в nginx контейнере).**
