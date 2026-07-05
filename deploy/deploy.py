#!/usr/bin/env python3
"""
ClinePass Proxy 部署脚本
使用 paramiko 上传文件并部署到远程服务器
"""

import paramiko
import os
import sys
import time

# 服务器配置
HOST = "154.219.110.138"
PORT = 22
USER = "root"
PASSWORD = "Zc2002119"

# 本地文件路径
LOCAL_BINARY = r"C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\bin\clinepass-proxy"
LOCAL_DEPLOY_SCRIPT = r"C:\Users\hayou\Desktop\OH-WorkSpace\_tools\clinepass-proxy\deploy\deploy.sh"

# 远程配置
REMOTE_DIR = "/tmp/clinepass-deploy"
API_KEY = ""  # 需要用户提供

def upload_file(sftp, local_path, remote_path):
    """上传文件到远程服务器"""
    print(f"上传: {local_path} -> {remote_path}")
    sftp.put(local_path, remote_path)
    print(f"✓ 上传成功")

def execute_command(ssh, command):
    """执行远程命令"""
    print(f"执行: {command}")
    stdin, stdout, stderr = ssh.exec_command(command)
    exit_status = stdout.channel.recv_exit_status()
    output = stdout.read().decode('utf-8')
    error = stderr.read().decode('utf-8')
    
    if exit_status != 0:
        print(f"✗ 命令失败 (exit status: {exit_status})")
        if error:
            print(f"错误: {error}")
        return False, output, error
    
    print(f"✓ 命令成功")
    return True, output, error

def main():
    # 检查 API Key
    if len(sys.argv) < 2:
        print("用法: python deploy.py <clinepass-api-key>")
        print("示例: python deploy.py sk-xxx...")
        sys.exit(1)
    
    api_key = sys.argv[1]
    
    print("=" * 50)
    print("ClinePass Proxy 部署脚本")
    print("=" * 50)
    print()
    print(f"目标服务器: {USER}@{HOST}:{PORT}")
    print(f"API Key: {api_key[:8]}...")
    print()
    
    # 创建 SSH 客户端
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    
    try:
        # 连接服务器
        print("1. 连接服务器...")
        ssh.connect(HOST, port=PORT, username=USER, password=PASSWORD, timeout=10)
        print("✓ 连接成功")
        print()
        
        # 创建 SFTP 客户端
        sftp = ssh.open_sftp()
        
        # 创建远程目录
        print("2. 创建远程目录...")
        execute_command(ssh, f"mkdir -p {REMOTE_DIR}")
        print()
        
        # 上传二进制文件
        print("3. 上传二进制文件...")
        upload_file(sftp, LOCAL_BINARY, f"{REMOTE_DIR}/clinepass-proxy")
        print()
        
        # 上传部署脚本
        print("4. 上传部署脚本...")
        upload_file(sftp, LOCAL_DEPLOY_SCRIPT, f"{REMOTE_DIR}/deploy.sh")
        print()
        
        # 设置执行权限
        print("5. 设置执行权限...")
        execute_command(ssh, f"chmod +x {REMOTE_DIR}/clinepass-proxy {REMOTE_DIR}/deploy.sh")
        print()
        
        # 执行部署脚本
        print("6. 执行部署脚本...")
        success, output, error = execute_command(ssh, f"cd {REMOTE_DIR} && ./deploy.sh {api_key}")
        if output:
            print(output)
        if error:
            print(error)
        print()
        
        if success:
            print("=" * 50)
            print("✓ 部署完成!")
            print("=" * 50)
            print()
            print("服务信息:")
            print(f"  - 安装目录: /opt/clinepass-proxy")
            print(f"  - 监听端口: 55991")
            print()
            print("常用命令:")
            print(f"  - 查看状态: ssh {USER}@{HOST} 'systemctl status clinepass-proxy'")
            print(f"  - 查看日志: ssh {USER}@{HOST} 'journalctl -u clinepass-proxy -f'")
            print(f"  - 重启服务: ssh {USER}@{HOST} 'systemctl restart clinepass-proxy'")
            print()
            print("API 端点:")
            print(f"  - http://{HOST}:55991/v1/chat/completions")
            print(f"  - http://{HOST}:55991/v1/models")
            print(f"  - http://{HOST}:55991/health")
            print()
            print("CherryStudio 配置:")
            print(f"  API 地址: http://{HOST}:55991/v1")
            print(f"  API Key:  任意值")
            print(f"  模型:     glm-5.2 / kimi-k2.7-code / deepseek-v4-pro 等")
        else:
            print("✗ 部署失败，请检查错误信息")
            sys.exit(1)
        
    except Exception as e:
        print(f"✗ 错误: {e}")
        sys.exit(1)
    finally:
        ssh.close()

if __name__ == "__main__":
    main()
