# Этап 2: Infrastructure as Code (Terraform)

На этом этапе мы переходим к **Terraform** — инструменту для управления инфраструктурой как кодом (IaC). В нашем случае мы будем использовать Terraform для того, чтобы поднять локальный Kubernetes кластер (k3d или kind), вместо того чтобы вводить команды руками.

## 1. Теория: Что нужно выучить

### Что такое Terraform?
Terraform позволяет описывать инфраструктуру (виртуальные машины, сети, базы данных, кластеры k8s) в виде декларативного кода на языке HCL (HashiCorp Configuration Language).

**Главные концепции Terraform:**
1. **Providers (Провайдеры):** Плагины, которые позволяют Terraform общаться с API различных систем (AWS, Google Cloud, Docker, Kubernetes, GitHub, GitLab). Проще говоря, провайдер "учит" Terraform, как создавать ресурсы в конкретном облаке или системе.
2. **Resources (Ресурсы):** Основной строительный блок Terraform. Это конкретный объект инфраструктуры, который мы хотим создать (например, `aws_instance` — виртуалка в AWS, или `kind_cluster` — локальный k8s кластер).
3. **State (Состояние - `terraform.tfstate`):** В отличие от Ansible, который заходит на сервер и проверяет текущее состояние по факту, Terraform хранит данные о том, *что именно он уже создал* в специальном файле. При каждом запуске он сравнивает свой код с этим `.tfstate` файлом и решает, что нужно добавить, изменить или удалить.
4. **Data Sources (Источники данных):** Позволяют получать информацию о ресурсах, которые были созданы вне Terraform или в других проектах (например, получить ID нужной VPC сети, чтобы положить туда наш кластер).
5. **Variables & Outputs:** Входные переменные (`variables.tf`) позволяют делать код переиспользуемым. Выходные значения (`outputs.tf`) позволяют получить какие-то полезные данные после создания (например, IP-адрес новой виртуалки).

**Жизненный цикл Terraform:**
- `terraform init` — скачивает провайдеры и инициализирует рабочую директорию.
- `terraform plan` — показывает "что будет сделано" (dry-run). Очень важно всегда смотреть план перед применением!
- `terraform apply` — применяет изменения (создает/меняет инфраструктуру).
- `terraform destroy` — удаляет всё, что было создано этим кодом.

---

## 2. Практика: Что конкретно сделать в коде

Наша цель — описать локальный кластер Kubernetes в Terraform. Мы будем использовать провайдер `tehcyx/kind`. Инструмент `kind` (Kubernetes in Docker) позволяет запускать легковесные узлы k8s внутри обычных Docker-контейнеров на твоей Ubuntu.

*Предварительное требование: Убедись, что на твоей Ubuntu установлены `docker`, `terraform` и бинарник `kind` (его можно скачать из [официального релиза](https://kind.sigs.k8s.io/docs/user/quick-start/)).*

### Структура папок
Создай папку `terraform` в корне проекта со следующей структурой:
```text
terraform/
├── main.tf
├── providers.tf
├── variables.tf
└── outputs.tf
```

### 1. `providers.tf`
Здесь мы указываем, какие провайдеры нам нужны и откуда их качать.
```hcl
terraform {
  required_version = ">= 1.0.0"

  required_providers {
    # Провайдер для создания кластера kind
    kind = {
      source  = "tehcyx/kind"
      version = "~> 0.2.0"
    }
  }
}

provider "kind" {
  # Настройки провайдера оставляем дефолтными
}
```

### 2. `variables.tf`
Сделаем имя кластера и версию Kubernetes настраиваемыми.
```hcl
variable "cluster_name" {
  type        = string
  description = "The name of the kind cluster"
  default     = "minion-bank-cluster"
}

variable "k8s_version" {
  type        = string
  description = "Kubernetes version image (e.g. kindest/node:v1.27.3)"
  default     = "kindest/node:v1.27.3"
}
```

### 3. `main.tf`
Самое главное — ресурс кластера. Мы укажем, что хотим одну Control Plane ноду (для управления) и две Worker ноды (для запуска наших приложений).

```hcl
resource "kind_cluster" "default" {
  name           = var.cluster_name
  node_image     = var.k8s_version
  wait_for_ready = true

  kind_config {
    kind        = "Cluster"
    api_version = "kind.x-k8s.io/v1alpha4"

    # Control plane node (Master)
    node {
      role = "control-plane"

      # Пробрасываем порты на хостовую машину Ubuntu (для Ingress)
      kubeadm_config_patches = [
        "kind: InitConfiguration\nnodeRegistration:\n  kubeletExtraArgs:\n    node-labels: \"ingress-ready=true\"\n"
      ]

      extra_port_mappings {
        container_port = 80
        host_port      = 80
        protocol       = "TCP"
      }
      extra_port_mappings {
        container_port = 443
        host_port      = 443
        protocol       = "TCP"
      }
    }

    # Worker nodes
    node {
      role = "worker"
    }
    
    node {
      role = "worker"
    }
  }
}
```

### 4. `outputs.tf`
Выводим путь к сгенерированному файлу (kubeconfig), чтобы мы могли потом подключаться к этому кластеру.
```hcl
output "kubeconfig_path" {
  description = "Path to the kubeconfig file for the created cluster"
  value       = kind_cluster.default.kubeconfig_path
}
```

---

## 3. Как проверить, что это работает

1. Перейди в терминале `bash` в директорию `terraform/`.
2. Инициализируй проект (Terraform скачает провайдер `tehcyx/kind`):
   ```bash
   terraform init
   ```
3. Посмотри, что собирается сделать Terraform:
   ```bash
   terraform plan
   ```
   Ты должен увидеть, что будет создан 1 ресурс (сам кластер).
4. Примени изменения:
   ```bash
   terraform apply -auto-approve
   ```
   *Этот процесс может занять пару минут, так как Kind скачает Docker-образ с Kubernetes.*
5. **Проверка работы:**
   Убедись, что кластер запущен и `kubectl` его видит:
   ```bash
   # Kind автоматически прописывает доступ в твой основной ~/.kube/config
   kubectl cluster-info
   kubectl get nodes
   ```
   У тебя должны отобразиться 3 ноды: 1 control-plane и 2 worker. Статус должен быть `Ready`.

**Как только твой кластер заработает, мы перейдем к Этапу 3 — будем разворачивать внутри него Ingress, БД, Kafka и писать манифесты для твоих Go/React приложений!**
