"""泡泡战士 CN0261 独立登录器

放到客户端安装目录运行（同目录需有 BubbleFighter.exe），
可指定远程服务端的 IP 与认证端口。

流程：
  1. 向服务端认证端口（默认 26102）发起 TLS 登录，换取一次性票据
  2. 在客户端目录写 login.ini（serverAddr / serverPort / id）
  3. 调 start_client.launch 启动客户端：改单实例互斥体 + 搬运票据
"""
import json
import os
import queue
import re
import socket
import ssl
import struct
import subprocess
import sys
import threading
import tkinter as tk
from pathlib import Path
from tkinter import ttk

from start_client import launch

APP_NAME = 'BubbleFighterCN0261Launcher'
CLIENT_IMAGE = 'BubbleFighter.exe'


def base_dir():
    if getattr(sys, 'frozen', False):
        return os.path.dirname(sys.executable)
    return os.path.dirname(os.path.abspath(__file__))


BASE = base_dir()
CLIENT_EXE = os.path.join(BASE, CLIENT_IMAGE)

DEFAULTS = {
    'host': '127.0.0.1',
    'auth_port': 26102,
    'account': '',
}


def config_path():
    root = os.environ.get('APPDATA') or os.path.expanduser('~')
    folder = os.path.join(root, APP_NAME)
    try:
        os.makedirs(folder, exist_ok=True)
    except Exception:
        return os.path.join(BASE, 'launcher-config.json')
    return os.path.join(folder, 'config.json')


def load_config():
    cfg = dict(DEFAULTS)
    try:
        with open(config_path(), 'r', encoding='utf-8') as f:
            cfg.update(json.load(f))
    except Exception:
        pass
    return cfg


def save_config(cfg):
    try:
        with open(config_path(), 'w', encoding='utf-8') as f:
            json.dump(cfg, f, ensure_ascii=False, indent=2)
    except Exception:
        pass


def receive_exact(connection, length):
    result = bytearray()
    while len(result) < length:
        data = connection.recv(length - len(result))
        if not data:
            raise ConnectionError('认证连接在响应完成前关闭')
        result.extend(data)
    return bytes(result)


ACCOUNT_PATTERN = r'[a-z0-9_]{3,32}'

# 注册固定建皮蛋（characters.character_type = 1）；玩家不再选首发角色。
STARTER_CHARACTER_TYPE = 1

AUTH_ERRORS = {
    'credentialsRejected': '账号或密码不正确，或账号已停用',
    'accountTaken': '该账号已被注册，请换一个',
    'nicknameTaken': '该角色名已被占用，请换一个',
    'inviteRequired': '邀请码不正确，请向服务器管理员索取',
    'serviceBusy': '认证服务繁忙或注册过于频繁，请稍后重试',
    'invalidRequest': '格式不合规：账号须为 3 至 32 位小写字母、数字或下划线；密码须为 8 至 128 个 UTF-8 字节；角色名须为 2 至 12 个字符且不含控制字符',
}


def describe_auth_error(response):
    return AUTH_ERRORS.get(response.get('error'), '认证服务暂时不可用')


def post_auth(host, port, payload):
    """向认证端口发送一条 JSON 请求并返回响应。

    自建服务器通常使用自签证书，这里不校验证书链，仅保证传输加密。
    """
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE

    body = json.dumps(payload, ensure_ascii=False).encode('utf-8')
    if len(body) > 4096:
        raise ValueError('认证请求过长')

    with socket.create_connection((host, port), timeout=10) as transport:
        with context.wrap_socket(transport, server_hostname=host) as connection:
            connection.sendall(struct.pack('<I', len(body)) + body)

            length = struct.unpack('<I', receive_exact(connection, 4))[0]
            if not (0 < length <= 4096):
                raise ValueError('认证响应长度不正确')

            return json.loads(receive_exact(connection, length).decode('utf-8'))


def check_credentials(account, password):
    """本地先校验一遍，避免把明显不合规的输入发到服务端。"""
    if not re.fullmatch(ACCOUNT_PATTERN, account):
        raise ValueError('账号须为 3 至 32 位小写字母、数字或下划线。')
    if not (8 <= len(password.encode('utf-8')) <= 128):
        raise ValueError('密码须为 8 至 128 个字节。')


def check_nickname(nickname):
    """本地先挡明显不合规的角色名，规则与原版一致（2–12 个 UTF-16 单元）。"""
    units = len(nickname.encode('utf-16-le')) // 2
    if units < 2 or units > 12:
        raise ValueError('角色名须为 2 至 12 个字符。')
    for character in nickname:
        code = ord(character)
        if code < 0x20 or 0x7F <= code <= 0x9F or code in (0x2028, 0x2029):
            raise ValueError('角色名不能包含控制字符。')


def register_account(host, port, account, password, invite_code, nickname='', character_type=1):
    """注册新账号并同时创建首个角色；服务端不返回票据，注册完仍需本人登录。"""
    payload = {
        'version': 1,
        'action': 'register',
        'accountName': account,
        'password': password,
    }
    if invite_code:
        payload['inviteCode'] = invite_code
    if nickname:
        payload['nickname'] = nickname
        payload['characterType'] = int(character_type)

    response = post_auth(host, port, payload)
    if response.get('success') is not True:
        raise ValueError(describe_auth_error(response))
    return response


def authenticate(host, port, account, password):
    """TLS 登录，返回服务端响应（含 loginTicket / gamePort）。"""
    response = post_auth(host, port, {
        'version': 1,
        'action': 'login',
        'accountName': account,
        'password': password,
    })

    if response.get('success') is not True:
        raise ValueError(describe_auth_error(response))

    if response.get('protocolVersion') != 717:
        raise ValueError('服务端协议版本不匹配（需要 717），请确认服务端版本')

    if not re.fullmatch('[0-9a-f]{64}', response.get('loginTicket', '')):
        raise ValueError('服务端返回的登录票据格式不正确')

    if not isinstance(response.get('gamePort'), int):
        raise ValueError('服务端返回的游戏端口不正确')

    return response


def running_clients():
    try:
        completed = subprocess.run(
            ['tasklist', '/FI', 'IMAGENAME eq ' + CLIENT_IMAGE, '/NH'],
            capture_output=True,
            text=True,
            creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0),
            timeout=15,
        )
    except Exception:
        return []
    return [line for line in (completed.stdout or '').splitlines()
            if CLIENT_IMAGE.lower() in line.lower()]


def start_game(host, response):
    token = response['loginTicket']
    game_port = int(response['gamePort'])
    login_path = os.path.join(BASE, 'login.ini')
    with open(login_path, 'w', encoding='utf-8') as f:
        f.write('serverAddr={}\nserverPort={}\nid={}\n'.format(host, game_port, token))

    letters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
    instance = letters[len(running_clients()) % len(letters)]
    return launch(Path(CLIENT_EXE), instance, launcher_identity=True)


class RegisterDialog(tk.Toplevel):
    """在登录器里直接注册账号：复用登录用的 TLS 通道与校验规则。"""

    def __init__(self, launcher, host, port):
        super().__init__(launcher)
        self.launcher = launcher
        self.host = host
        self.port = port
        self.busy = False

        self.title('注册新账号')
        self.geometry('440x320')
        self.resizable(False, False)
        self.transient(launcher)
        self.grab_set()

        self.var_account = tk.StringVar()
        self.var_password = tk.StringVar()
        self.var_confirm = tk.StringVar()
        self.var_nickname = tk.StringVar()
        self.var_invite = tk.StringVar()
        self.var_status = tk.StringVar(value='服务器：{}:{}'.format(host, port))

        frame = ttk.Frame(self, padding=16)
        frame.pack(fill='both', expand=True)

        ttk.Label(frame, text='账号').grid(row=0, column=0, sticky='e', padx=(0, 8), pady=4)
        account_entry = ttk.Entry(frame, textvariable=self.var_account, width=30)
        account_entry.grid(row=0, column=1, sticky='we', pady=4)

        ttk.Label(frame, text='密码').grid(row=1, column=0, sticky='e', padx=(0, 8), pady=4)
        ttk.Entry(frame, textvariable=self.var_password, show='●', width=30).grid(
            row=1, column=1, sticky='we', pady=4)

        ttk.Label(frame, text='确认密码').grid(row=2, column=0, sticky='e', padx=(0, 8), pady=4)
        ttk.Entry(frame, textvariable=self.var_confirm, show='●', width=30).grid(
            row=2, column=1, sticky='we', pady=4)

        ttk.Label(frame, text='角色名').grid(row=3, column=0, sticky='e', padx=(0, 8), pady=4)
        ttk.Entry(frame, textvariable=self.var_nickname, width=30).grid(
            row=3, column=1, sticky='we', pady=4)

        ttk.Label(frame, text='邀请码').grid(row=4, column=0, sticky='e', padx=(0, 8), pady=4)
        ttk.Entry(frame, textvariable=self.var_invite, width=30).grid(
            row=4, column=1, sticky='we', pady=4)
        hint = ttk.Label(
            frame,
            text='角色注册时固定为皮蛋，进入游戏后自行变更\n'
                 '邀请码在服务器开放注册时可不填\n'
                 '每个 IP 每小时最多注册 5 次',
            font=('Microsoft YaHei UI', 8), foreground='#888',
            justify='left', anchor='w', wraplength=360)
        hint.grid(row=5, column=0, columnspan=2, sticky='we', pady=(2, 0))

        self.button = ttk.Button(frame, text='注册', command=self.submit)
        self.button.grid(row=6, column=0, columnspan=2, sticky='we', pady=(12, 8))
        ttk.Label(frame, textvariable=self.var_status, wraplength=380,
                  foreground='#555').grid(row=7, column=0, columnspan=2, sticky='w')

        frame.columnconfigure(1, weight=1)
        self.bind('<Return>', lambda _e: self.submit())
        account_entry.focus_set()

    def set_busy(self, value):
        self.busy = value
        self.button.state(['disabled'] if value else ['!disabled'])

    def submit(self):
        if self.busy:
            return
        account = self.var_account.get().strip()
        password = self.var_password.get()
        confirm = self.var_confirm.get()
        nickname = self.var_nickname.get().strip()
        invite = self.var_invite.get().strip()

        try:
            check_credentials(account, password)
            check_nickname(nickname)
        except ValueError as error:
            self.var_status.set(str(error))
            return
        if password != confirm:
            self.var_status.set('两次输入的密码不一致。')
            return

        self.set_busy(True)
        self.var_status.set('正在注册……')
        threading.Thread(
            target=self.worker,
            args=(account, password, invite, nickname, STARTER_CHARACTER_TYPE),
            daemon=True,
        ).start()

    def worker(self, account, password, invite, nickname, character_type):
        try:
            register_account(self.host, self.port, account, password, invite,
                             nickname, character_type)
        except socket.timeout:
            self.after(0, lambda: self.finish('连接超时：请检查服务器 IP、端口和防火墙。'))
        except ValueError as error:
            self.after(0, lambda: self.finish(str(error)))
        except Exception as error:
            self.after(0, lambda: self.finish(
                '注册失败（{}）：{}'.format(type(error).__name__, error)))
        else:
            self.after(0, lambda: self.succeeded(account))

    def finish(self, message):
        self.set_busy(False)
        self.var_status.set(message)

    def succeeded(self, account):
        self.set_busy(False)
        # 不自动登录：注册只负责建号，登录仍由本人在主界面点击。
        self.launcher.var_account.set(account)
        self.launcher.var_password.set('')
        self.launcher.var_status.set('注册成功，账号与角色已创建，请使用新账号登录。')
        self.destroy()


class Launcher(tk.Tk):
    def __init__(self):
        super().__init__()
        self.title('泡泡战士 CN0261 登录器 v0.1.0 By:rurudoo')
        self.geometry('540x330')
        self.resizable(False, False)

        self.cfg = load_config()
        self.events = queue.Queue()
        self.busy = False

        self.var_host = tk.StringVar(value=self.cfg['host'])
        self.var_port = tk.StringVar(value=str(self.cfg['auth_port']))
        self.var_account = tk.StringVar(value=self.cfg['account'])
        self.var_password = tk.StringVar()
        self.var_status = tk.StringVar()

        frame = ttk.Frame(self, padding=20)
        frame.pack(fill='both', expand=True)

        ttk.Label(frame, text='泡泡战士 · CN0261', font=('Microsoft YaHei UI', 17)).grid(
            row=0, column=0, columnspan=4, sticky='w', pady=(0, 12))

        ttk.Label(frame, text='服务器').grid(row=1, column=0, sticky='e', padx=(0, 8), pady=5)
        ttk.Entry(frame, textvariable=self.var_host, width=24).grid(
            row=1, column=1, sticky='w', pady=5)
        ttk.Label(frame, text='认证端口').grid(row=1, column=2, sticky='e', padx=(12, 8), pady=5)
        ttk.Entry(frame, textvariable=self.var_port, width=10).grid(
            row=1, column=3, sticky='w', pady=5)

        ttk.Label(frame, text='账号').grid(row=2, column=0, sticky='e', padx=(0, 8), pady=5)
        self.account_entry = ttk.Entry(frame, textvariable=self.var_account, width=40)
        self.account_entry.grid(row=2, column=1, columnspan=3, sticky='we', pady=5)

        ttk.Label(frame, text='密码').grid(row=3, column=0, sticky='e', padx=(0, 8), pady=5)
        password_entry = ttk.Entry(frame, textvariable=self.var_password, show='●', width=40)
        password_entry.grid(row=3, column=1, columnspan=3, sticky='we', pady=5)

        actions = ttk.Frame(frame)
        actions.grid(row=4, column=0, columnspan=4, sticky='we', pady=(16, 10))
        actions.columnconfigure(0, weight=1)
        self.button = ttk.Button(actions, text='登录并启动游戏', command=self.submit)
        self.button.grid(row=0, column=0, sticky='we')
        self.register_button = ttk.Button(actions, text='注册新账号', command=self.open_register)
        self.register_button.grid(row=0, column=1, sticky='w', padx=(8, 0))

        ttk.Label(frame, textvariable=self.var_status, wraplength=490, foreground='#555').grid(
            row=5, column=0, columnspan=4, sticky='w')
        ttk.Label(frame,
                  text='账号可在本登录器注册；服务端地址填运行服务端那台机器的 IP。',
                  font=('Microsoft YaHei UI', 8),
                  foreground='#888').grid(row=6, column=0, columnspan=4, sticky='w', pady=(10, 0))

        frame.columnconfigure(1, weight=1)

        self.protocol('WM_DELETE_WINDOW', self.destroy)
        self.after(100, self.poll)

        if not os.path.isfile(CLIENT_EXE):
            self.var_status.set('未检测到 BubbleFighter.exe，请把本程序放到客户端安装目录内运行。')
            self.button.state(['disabled'])
            return

        self.var_status.set('准备就绪。')
        self.account_entry.focus_set()
        self.bind('<Return>', lambda _e: self.submit())

    def log(self, text):
        self.events.put(text)

    def poll(self):
        try:
            while True:
                self.var_status.set(self.events.get_nowait())
        except queue.Empty:
            pass
        self.after(100, self.poll)

    def set_busy(self, value):
        self.busy = value
        state = ['disabled'] if value else ['!disabled']
        self.button.state(state)
        self.register_button.state(state)

    def open_register(self):
        if self.busy:
            return
        host = self.var_host.get().strip()
        try:
            port = int(self.var_port.get().strip())
        except ValueError:
            self.var_status.set('认证端口必须是数字。')
            return
        if not host:
            self.var_status.set('请填写服务器 IP 或域名。')
            return
        RegisterDialog(self, host, port)

    def submit(self):
        if self.busy:
            return

        if not os.path.isfile(CLIENT_EXE):
            self.var_status.set('未检测到 BubbleFighter.exe，请把本程序放到客户端安装目录内运行。')
            return

        host = self.var_host.get().strip()
        account = self.var_account.get().strip()
        password = self.var_password.get()

        try:
            port = int(self.var_port.get().strip())
        except ValueError:
            self.var_status.set('认证端口必须是数字。')
            return

        if not host:
            self.var_status.set('请填写服务器 IP 或域名。')
            return

        if not re.fullmatch('[a-z0-9_]{3,32}', account):
            self.var_status.set('账号须为 3 至 32 位小写字母、数字或下划线。')
            return

        if not (8 <= len(password.encode('utf-8')) <= 128):
            self.var_status.set('密码须为 8 至 128 个字节。')
            return

        self.cfg.update({'host': host, 'auth_port': port, 'account': account})
        save_config(self.cfg)
        self.var_password.set('')

        self.set_busy(True)
        self.var_status.set('正在连接 {}:{} ……'.format(host, port))
        threading.Thread(
            target=self.worker,
            args=(host, port, account, password),
            daemon=True,
        ).start()

    def worker(self, host, port, account, password):
        try:
            response = authenticate(host, port, account, password)
            self.log('认证成功，正在启动客户端……')
            start_game(host, response)
            self.log('客户端已启动。')
        except socket.timeout:
            self.log('连接超时：请检查服务器 IP、端口和防火墙。')
        except ConnectionRefusedError:
            self.log('连接被拒绝：服务端未启动，或认证端口未放行。')
        except ssl.SSLError as error:
            self.log('TLS 握手失败：{}'.format(error))
        except (ValueError, ConnectionError) as error:
            self.log(str(error))
        except Exception as error:
            self.log('启动失败（{}）：{}'.format(type(error).__name__, error))
        finally:
            self.after(0, lambda: self.set_busy(False))


if __name__ == '__main__':
    if not os.path.isfile(CLIENT_EXE):
        try:
            import ctypes
            ctypes.windll.user32.MessageBoxW(
                None,
                '未检测到 BubbleFighter.exe。\n\n请把本程序放到客户端安装目录内运行。',
                '泡泡战士 CN0261 登录器 v0.1.0 By:rurudoo',
                64,
            )
        except Exception:
            pass
        raise SystemExit(1)
    Launcher().mainloop()
