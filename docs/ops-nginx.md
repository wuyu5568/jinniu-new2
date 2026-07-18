# Nginx + HTTPS（L1：go.wulid.com）

约定：

| 路径 | 内容 |
|------|------|
| `/` | 用户端 taurus |
| `/admin/` | 管理端 dapp-admin |
| `/api/` | 反代本机 `127.0.0.1:8000` |

域名：`go.wulid.com`、`www.go.wulid.com` → EC2 公网 IP。

## 1. 安全组

入站放行 **TCP 80**、**TCP 443**（来源可先 `0.0.0.0/0`）。8000 可改为仅本机（安全组去掉公网 8000，只留 Nginx）。

## 2. 安装 Nginx / Certbot（EC2）

```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
sudo mkdir -p /var/www/jinniu/html/admin
sudo chown -R ubuntu:ubuntu /var/www/jinniu
```

## 3. 上传静态资源（本机 PowerShell）

```powershell
# 用户端：先传到 /tmp，再 rsync（务必 --exclude admin，否则会删掉管理端）
scp -i C:\Users\Lenovo\Desktop\github\sssss.pem -r C:\Users\Lenovo\Desktop\github\taurus\dist\* ubuntu@54.150.130.182:/tmp/taurus-dist/
# EC2:
# sudo rsync -a --delete --exclude admin /tmp/taurus-dist/ /var/www/jinniu/html/
# sudo chown -R www-data:www-data /var/www/jinniu/html && sudo chmod -R a+rX /var/www/jinniu/html

# 管理端（publicPath=/admin）
scp -i C:\Users\Lenovo\Desktop\github\sssss.pem -r C:\Users\Lenovo\Desktop\github\dapp-admin\dist\* ubuntu@54.150.130.182:/tmp/admin-dist/
# EC2:
# sudo mkdir -p /var/www/jinniu/html/admin
# sudo rsync -a --delete /tmp/admin-dist/ /var/www/jinniu/html/admin/
# sudo chown -R www-data:www-data /var/www/jinniu/html/admin && sudo chmod -R a+rX /var/www/jinniu/html/admin
```

## 4. Nginx 站点（先 HTTP，再签证书）

```bash
sudo nano /etc/nginx/sites-available/jinniu
```

内容：

```nginx
server {
    listen 80;
    server_name go.wulid.com www.go.wulid.com;

    root /var/www/jinniu/html;
    index index.html;

    client_max_body_size 20m;

    location /api/ {
        proxy_pass http://127.0.0.1:8000/api/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 360s;
        proxy_send_timeout 360s;
    }

    location /admin {
        try_files $uri $uri/ /admin/index.html;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

启用：

```bash
sudo ln -sf /etc/nginx/sites-available/jinniu /etc/nginx/sites-enabled/jinniu
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

本机测：

```bash
curl -sI http://go.wulid.com/ | head -5
curl -s http://go.wulid.com/api/ | head -c 200
curl -s http://127.0.0.1:8000/health
```

健康检查仍在后端根路径，可另加：

```nginx
    location = /health {
        proxy_pass http://127.0.0.1:8000/health;
    }
```

## 5. Let’s Encrypt

```bash
sudo certbot --nginx -d go.wulid.com -d www.go.wulid.com
```

按提示选 redirect HTTP→HTTPS。续期：`sudo certbot renew --dry-run`。

## 6. 浏览器验收

- 用户端：https://go.wulid.com/
- 管理端：https://go.wulid.com/admin/
- API：登录管理端看首页打款提示

前端 `url.js` / `VITE_API` 须为 `https://go.wulid.com`（已按此打包）。

## 7. 限流 / 连接限制 / 基础加固（仅金牛）

仓库参考配置：

| 文件 | 部署位置 |
|------|----------|
| `docs/nginx/jinniu-protect.conf` | `/etc/nginx/conf.d/jinniu-protect.conf` |
| `docs/nginx/jinniu.site.conf` | `/etc/nginx/sites-available/jinniu`（合并/覆盖时保留 Certbot SSL 段） |

策略（按公网 IP，令牌桶）：

| 路径 | 速率 | 突发 | 同时连接 |
|------|------|------|----------|
| `/api/` | 10r/s | 20 | 20 |
| 页面/静态 | 30r/s | 50 | 50（整站） |
| `/health` | 不走 API 限流区 | — | 走整站连接上限 |

超限返回 **429**。另：`server_tokens off`；仅允许 `GET/POST/HEAD/OPTIONS`（其余 **405**）；`client_max_body_size 20m`；header/body 超时 10s。

改完后：

```bash
sudo nginx -t && sudo systemctl reload nginx
```

不影响 `java.wulid.com`。
