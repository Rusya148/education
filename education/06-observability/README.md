# Этап 6: Наблюдаемость (Observability) и Логирование

Инфраструктура работает, код автоматически собирается и деплоится. Но если прямо сейчас пользователь `Minion Bank` нажмет "Перевести деньги" и получит ошибку 500 — *как мы об этом узнаем?* Куда нам смотреть, чтобы понять: упала база, кончилась память на поде или баг в коде `Go`?

Для этого нам нужен мониторинг и централизованные логи.

## 1. Теория: Что нужно выучить

### Метрики (Prometheus & Grafana)
**Prometheus:** Инструмент для сбора числовых метрик (CPU, RAM, количество 200/500 ответов от бэкенда). Он работает по модели **Pull** — он сам регулярно (каждые 15 секунд) ходит по эндпоинтам твоих сервисов (например `/metrics`) и "сгребает" оттуда данные.
**Grafana:** Красивый дашборд (графики, диаграммы), который рисует картинки на основе данных, собранных Прометеусом.

**Prometheus Operator (kube-prometheus-stack):** 
Это способ развернуть весь этот стек в k8s "по правильному". Оператор добавляет в k8s новые кастомные ресурсы (CRD) — например `ServiceMonitor`. Когда ты описываешь ServiceMonitor, оператор сам перенастраивает Prometheus, чтобы тот начал ходить и собирать метрики с твоего Go-приложения.

### Логи (Loki & Promtail)
Логи контейнеров (вывод `stdout` / `stderr`) в K8s живут ровно столько же, сколько сам под. Если под умер — логи пропали.
**Promtail:** Агент, который ставится на каждую ноду (через DaemonSet). Он читает логи *всех* контейнеров из папки `/var/log/containers` и пересылает их в централизованное хранилище. По модели **Push**.
**Loki:** Хранилище логов от Grafana (младший брат ELK-стека). Оно берет логи от Promtail и позволяет искать по ним в интерфейсе Grafana (например `{app="backend-app"} |= "error"`, чтобы найти ошибки).

---

## 2. Практика: Что конкретно сделать в коде

### 2.1 Инструментирование Go-приложения
В бэкенде нужно подключить библиотеку `github.com/prometheus/client_golang` и выставить эндпоинт `/metrics`.

*(В коде твоего Go сервиса, `main.go`)*:
```go
import (
	"log"
	"net/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Создаем метрику-счетчик (Counter)
var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Общее количество обработанных HTTP запросов",
	},
	[]string{"status", "path"},
)

func init() {
	// Регистрируем метрику при старте
	prometheus.MustRegister(httpRequestsTotal)
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	// Увеличиваем счетчик при каждом запросе: статус=200, путь=/api/data
	httpRequestsTotal.WithLabelValues("200", r.URL.Path).Inc()
	w.Write([]byte("Hello Minion Bank!"))
}

func main() {
    // Твои основные роуты
	http.HandleFunc("/api/data", myHandler)
	
    // Роут для Прометеуса (выдает внутренние данные Go и наши кастомные счетчики)
	http.Handle("/metrics", promhttp.Handler())
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### 2.2 Установка Kube-Prometheus Stack
Устанавливаем Prometheus, Grafana и AlertManager (для уведомлений в телеграм/slack) через официальный Helm-чарт.

```bash
# Создаем отдельное окружение (namespace) для мониторинга
kubectl create namespace monitoring

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack -n monitoring
```

### 2.3 Говорим Прометеусу собирать наши метрики
Чтобы Prometheus нашел твой бэкенд, создадим `ServiceMonitor` манифест (`kubernetes/backend-servicemonitor.yaml`):

```yaml
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: backend-monitor
  labels:
    release: kube-prometheus-stack # Важный label! Оператор должен понимать, что этот монитор принадлежит ему
spec:
  selector:
    matchLabels:
      app: backend # Совпадает с label нашего Service
  endpoints: # Откуда собирать
  - port: web # Имя порта из backend-service.yaml
    path: /metrics
    interval: 15s
```
*(Примени через `kubectl apply -f kubernetes/backend-servicemonitor.yaml`)*.

### 2.4 Установка Loki и Promtail
Графана у нас уже есть, так что ставим только сборщик (Loki) и агенты (Promtail):

```bash
helm repo add grafana https://grafana.github.io/helm-charts
# Устанавливаем и отключаем встроенную графану, так как она уже установлена прометеусом выше
helm install loki grafana/loki-stack --set promtail.enabled=true --set grafana.enabled=false -n monitoring
```

### 2.5 Подключение Loki к Grafana
Графана (из prometheus-stack) живет в неймспэйсе `monitoring`.
1. Пробрось порт:
   ```bash
   kubectl port-forward svc/kube-prometheus-stack-grafana -n monitoring 3000:80
   ```
2. Открой `http://localhost:3000` в браузере.
   *(Дефолтный логин: `admin`, пароль можно подсмотреть командой `kubectl get secret --namespace monitoring kube-prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 --decode`)*.
3. Перейди в **Connections -> Add new connection -> Loki**.
4. В поле URL укажи внутренний k8s DNS адрес самого сервиса Loki: `http://loki:3100`.
5. Нажми **Save & Test**.

---

## 3. Как проверить, что это работает

### Проверка метрик:
1. В Grafana нажми на **Explore**.
2. Источник данных: **Prometheus**.
3. Введи запрос: `rate(http_requests_total[5m])` (Этот график покажет тебе количество запросов в секунду к твоему Go-бэкенду).
   *Заметь: `http_requests_total` появится только после того, как ты сделаешь пару `curl`-ов на твой бэкенд, чтобы счетчик `Inc()` сработал!*
4. По желанию: Создай Dashboard и выведи туда статистику использования CPU твоими нодами: `node_cpu_seconds_total`.

### Проверка логов:
1. В той же Grafana в меню **Explore**.
2. Измени источник данных на **Loki**.
3. Нажми на **Label filters** -> выбери `app` = `backend`.
4. Нажми `Run Query`. 
5. Ниже загрузится "бесконечная" лента логов твоего бэкенда! Теперь, если под упадет, его логи всё равно останутся в Loki.

---
**Поздравляю! Ты прошел через все этапы DevOps трансформации проекта!** Твой `Minion Bank` теперь описан кодом, контейнеризован, отказоустойчиво крутится в K8s, безопасно хранит секреты в Vault, автоматически собирается через CI/CD и прозрачен для отладки благодаря Prom/Loki.
