# Nexd Proxy in Kubernetes

```yaml
apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: null
  name: demo1
spec: {}
status: {}
apiVersion: v1
data:
  password: ...
  username: ...
kind: Secret
metadata:
  creationTimestamp: null
  name: nexodus-credentials
  namespace: demo1
apiVersion: v1
data:
  private.key: ...
  public.key: ...
kind: Secret
metadata:
  creationTimestamp: null
  name: wireguard-keys
  namespace: demo1
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: demo1
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      volumes:
      - name: shared-data
        emptyDir: {}
      initContainers:
      - name: init-nginx
        image: nginx
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        command: ["sh", "-c", "echo \"Hello from $POD_NAME\" > /usr/share/nginx/html/index.html"]
        volumeMounts:
        - name: shared-data
          mountPath: /usr/share/nginx/html
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        volumeMounts:
        - name: shared-data
          mountPath: /usr/share/nginx/html
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: demo1
spec:
  selector:
    app: nginx
  ports:
  - name: http
    port: 80
    targetPort: 80
  type: ClusterIP

apiVersion: apps/v1
kind: Deployment
metadata:
  name: nexd-proxy
  namespace: demo1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nexd-proxy
  template:
    metadata:
      labels:
        app: nexd-proxy
    spec:
      containers:
      - name: my-container
        image: quay.io/nexodus/nexd
        command: ["sh"]
        args: ["-c", "ln -s /etc/wireguard/private.key /private.key; ln -s /etc/wireguard/public.key /public.key; nexd proxy --ingress tcp:80:nginx-service.demo1.svc.cluster.local:80 https://qa.nexodus.io"]
        env:
        - name: NEXD_USERNAME
          valueFrom:
            secretKeyRef:
              name: nexodus-credentials
              key: username
        - name: NEXD_PASSWORD
          valueFrom:
            secretKeyRef:
              name: nexodus-credentials
              key: password
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        volumeMounts:
        - name: wireguard-keys
          mountPath: /etc/wireguard/
      volumes:
      - name: wireguard-keys
        secret:
          secretName: wireguard-keys
```