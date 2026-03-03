# Этап 4: Шаблонизация K8s манифестов (Helm для своих проектов)

Писать "сырые" манифесты Kubernetes — это нормально для первого знакомства (как мы делали на Неделе 3). Но когда у тебя появляется потребность запускать код в **dev**-окружении для тестирования и в **prod**-окружении для реальных пользователей, возникает проблема. 

Тебе придется копировать файлы, менять названия доменов (с `dev.minionbank` на `minionbank`), менять количество реплик и так далее. DRY (Don't Repeat Yourself) нарушается.

Решение проблемы — создание собственного **Helm Chart** для твоего приложения.

## 1. Теория: Что нужно выучить

### Анатомия Helm-чарта
Helm позволяет брать абстрактные манифесты и подставлять туда переменные. Структура чарта:
```text
minion-backend/
├── Chart.yaml          # Метаданные (название чарта, версия)
├── values.yaml         # Значения по умолчанию для шаблонов
├── templates/          # Сами K8s манифесты с переменными {{ .Values.xxx }}
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ingress.yaml
├── values-dev.yaml     # (Твой кастомный файл) Значения для DEV-окружения
└── values-prod.yaml    # (Твой кастомный файл) Значения для PROD-окружения
```

- **Рендеринг:** Во время установки (или обновления), Helm берет `values.yaml` (и перекрывает его через `-f values-dev.yaml`), применяет их к шаблонам в папке `templates/`, генерирует чистый YAML k8s и отправляет его в кластер.

---

## 2. Практика: Что конкретно сделать в коде

### 2.1 Создание структуры чарта
Давай создадим чарт для бэкенда на твоей Ubuntu-машине (предполагаем, что утилита `helm` уже установлена):

```bash
# Создаст болванку чарта (там много лишнего)
helm create minion-backend
# Удаляем весь стандартный мусор из шаблонов
rm -rf minion-backend/templates/*
```

### 2.2 Пишем шаблоны (`templates/`)
Возьми свои манифесты из прошлого урока и перенеси их в `minion-backend/templates/`, но замени жестко заданные вещи (хардкод) на переменные.

Вот как теперь будет выглядеть `minion-backend/templates/deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # Используем стандартные функции helm (include) чтобы генерировать имя
  name: {{ .Release.Name }}-backend
  labels:
    app: backend
spec:
  # Количество реплик теперь настраивается
  replicas: {{ .Values.replicaCount }}
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
        # Собираем путь к образу динамически
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: "{{ .Values.database.host }}"
```

И `minion-backend/templates/ingress.yaml`:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Release.Name }}-ingress
spec:
  rules:
  - host: {{ .Values.ingress.host }}
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: {{ .Release.Name }}-backend
            port:
              number: 80
```

### 2.3 Пишем слои конфигурации (`values`)

Создай **`minion-backend/values.yaml`** — базовый файл по умолчанию.
```yaml
replicaCount: 1
image:
  repository: rusya148/minion-backend
  pullPolicy: IfNotPresent
  tag: "latest"

database:
  host: "192.168.1.X" # Дефолтная локальная база на твоем Ubuntu-хосте

ingress:
  host: "local.minionbank.local"
```

Теперь создай **`minion-backend/values-dev.yaml`** — для ветки разработки/тестирования.
```yaml
replicaCount: 1
image:
  tag: "develop-branch-hash"
ingress:
  host: "dev.minionbank.local"
```

И **`minion-backend/values-prod.yaml`** — для боевого окружения!
```yaml
replicaCount: 3
image:
  tag: "stable-v1.0.0"
ingress:
  host: "minionbank.local"
```

---

## 3. Как проверить, что это работает

1. **Dry Run (Рендеринг):** Убедимся, что Helm правильно склеивает YAML, не применяя его в кластер:
   ```bash
   helm template my-dev minion-backend -f minion-backend/values-dev.yaml
   ```
   Ты должен увидеть готовый, красивый Kubernetes YAML без фигурных скобок `{{ }}`, в котором домен будет именно `dev.minionbank.local`, а не дефолтный.

2. **Деплой DEV:**
   Создадим неймспэйс и задеплоим чарт:
   ```bash
   kubectl create namespace dev
   helm upgrade --install backend-dev ./minion-backend -n dev -f ./minion-backend/values-dev.yaml
   ```

3. **Деплой PROD:**
   В соседнем неймспэйсе, с другими настройками (из одного и того же кода чарта!):
   ```bash
   kubectl create namespace prod
   helm upgrade --install backend-prod ./minion-backend -n prod -f ./minion-backend/values-prod.yaml
   ```

4. **Проверка:**
   ```bash
   kubectl get pods -n dev
   kubectl get pods -n prod
   ```
   У тебя должен работать 1 под в `dev` и целых 3 пода в `prod` (в соответствии с настройками `replicaCount` из файлов `values`).

**Отлично! Твоя инфраструктура готова к гибкому развертыванию. В следующем шаге (ArgoCD) мы перестанем писать команду `helm upgrade` руками, а заставим кластер *сам* забирать этот чарт прямо из Git.**
