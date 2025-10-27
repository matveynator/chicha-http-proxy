<img src="https://github.com/matveynator/chicha-http-proxy/blob/main/chicha-http-proxy.png?raw=true" alt="chicha-http-proxy" width="100%" align="right" />

# **chicha-http-proxy: The Easiest Way to Mirror Websites**

Chicha HTTP Proxy is a simple yet powerful tool for creating mirrors of websites or services. It forwards traffic to a target URL, supports both HTTP and HTTPS, and can automatically manage SSL certificates from Let's Encrypt. Built with Go, it’s lightweight, fast, and incredibly easy to use.

---

### **Why Choose chicha-http-proxy?**
- **Create Mirrors in Minutes**: Quickly mirror websites with minimal configuration.  
- **Blazing Fast**: Powered by Go, mirrors handle high traffic effortlessly.  
- **Automatic SSL**: Easily enable HTTPS with Let's Encrypt certificates.  
- **Beginner-Friendly**: Simple setup—just point to the target URL.  
- **Cross-Platform**: Works on Linux, Windows, macOS, and more.

---

### **Important Notes on Ports and Privileges**

- **Ports for SSL Certificates**:  
  When using the `--domain` option (for HTTPS with Let's Encrypt):  
  - HTTP is **always forced to port 80** for certificate validation (cannot be changed).  
  - HTTPS defaults to port 443, but you can set a custom HTTPS port.  

  If you use port 80 on Linux, you’ll need `sudo` because it’s a privileged port.

- **Custom Ports Without SSL**:  
  If you don't specify a domain (no SSL), you can use any HTTP port (e.g., 8080) without needing `sudo`.

---

### **Downloads**

- **Linux**: [AMD64](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/linux/amd64/chicha-http-proxy), [ARM64](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/linux/arm64/chicha-http-proxy)  
- **Windows**: [AMD64](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/windows/amd64/chicha-http-proxy.exe), [ARM64](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/windows/arm64/chicha-http-proxy.exe)  
- **macOS**: [Intel](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/mac/amd64/chicha-http-proxy), [M1/M2](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/mac/arm64/chicha-http-proxy)

[More platforms here](https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui).

---

### **Installation on Linux/macOS**

```bash
sudo curl https://files.zabiyaka.net/chicha-http-proxy/latest/no-gui/linux/amd64/chicha-http-proxy -o /usr/local/bin/chicha-http-proxy
sudo chmod +x /usr/local/bin/chicha-http-proxy
```

For Windows, download the `.exe` and add it to your `PATH`.

---

### **Usage Examples**

#### **1. Create a Simple HTTP Mirror (No SSL)**:
This creates an HTTP mirror of `twochicks.ru` on port 8080. No admin privileges are required:
```bash
chicha-http-proxy --http-port=8080 --target-url=https://twochicks.ru
```

#### **2. Create a Secure HTTPS Mirror with Automatic SSL**:
When you specify `--domain`, the tool automatically gets an SSL certificate from Let's Encrypt.  
HTTP is forced to port 80 for validation, and HTTPS defaults to port 443:
```bash
sudo chicha-http-proxy --domain=your-domain.com --target-url=https://twochicks.ru
```

#### **3. Use a Custom HTTPS Port**:
Keep HTTP on port 80 for validation, but serve HTTPS on a custom port (e.g., 8443):
```bash
sudo chicha-http-proxy --domain=your-domain.com --https-port=8443 --target-url=https://twochicks.ru
```

---

### **Systemd Setup for Autostart**

1. **Create a Service File**:
   ```bash
   sudo vim /etc/systemd/system/chicha-http-proxy.service
   ```

2. **Add the Following Configuration**:
   ```
[Unit]
Description=Chicha HTTP Proxy
After=network.target

[Service]
User=root
ExecStart=/usr/local/bin/chicha-http-proxy -domain=DOMAIN.COM -target-url=https://127.0.0.1:8080
Restart=on-failure

[Install]
WantedBy=multi-user.target
   ```

3. **Enable and Start the Service**:
   ```bash
   systemctl daemon-reload
   systemctl enable chicha-http-proxy
   systemctl start chicha-http-proxy
   systemctl status chicha-http-proxy
   ```

---

### **Key Features**
- **Effortless Website Mirroring**: Create fast, functional mirrors in minutes.  
- **Blazing Fast Performance**: Optimized for high traffic with minimal overhead.  
- **Automatic HTTPS**: Free SSL certificates with Let's Encrypt.  
- **Flexible Ports**:  
  - With `--domain`: HTTP fixed at 80, HTTPS customizable.  
  - Without `--domain`: Any HTTP port can be used.  
- **Cross-Platform Support**: Works on Linux, Windows, macOS, and more.

---

Chicha HTTP Proxy is the easiest and fastest way to mirror websites with minimal setup. Download it now and start building secure, high-performance mirrors today!
