"""高分辨率扩展：照搬 FantasyTale.Launcher 的 cn-717 分辨率插件（MIT）补丁点。

原理（对应插件 bubblefighter.fantasy.resolution-extension v0.2.6 的 cn-717 描述符）：
客户端原生分辨率表固定在 VA 0xFDE9F8（7 项 × 8 字节，宽/高各 u32，表尾 0xFDEA30）。
把「用到这张表的 5 处指令」的立即数改指向一张更大的表，并 NOP 掉「索引 > 6 就跳走」
的计数检查。默认与插件一致：不动显示判定函数（`FORCE_PREDICATE` 打开后才写恒真）。

不改文件：全部通过 CreateProcess 挂起期间 WriteProcessMemory 完成，进程恢复即生效。
任何一处校验对不上都会整体放弃，客户端照常启动。

用法（静态自检，不启动游戏）：
    python hidpi.py path\\to\\BubbleFighter.exe
"""
import ctypes
import struct
from ctypes import wintypes

IMAGE_BASE = 0x400000
TIME_DATE_STAMP = 0x5582B5CD                     # 2015-06-18T12:13:01Z
IMAGE_SIZES = (0x01090000, 0x01093000)
TABLE_RVA = 0xBDE9F8
GUARD_RVA = 0x781A50
GUARD = bytes.fromhex('8B810C0700002B8108070000')
ORIGINAL_MODES = [(800, 600), (1024, 768), (1152, 864), (1280, 800),
                  (1280, 960), (1366, 768), (1600, 900)]
MINIMUM_WIDTH = 1024
MINIMUM_HEIGHT = 768
MAXIMUM_ASPECT_MILLI = 1780                      # 宽×1000 < 高×1780（≈16:9）
CUSTOM_MODES = []                                # 与插件默认一致：只靠 D3D 枚举，不硬塞档位

# 插件默认不动显示判定函数（它只在用户填了自定义分辨率时才重定向菜单那次调用）。
# 若实测发现菜单里看不到新档位，把它改成 True：判定函数会被写成恒真（mov al,1; ret 8）。
FORCE_PREDICATE = False

TABLE_OFFSET_BEGIN = 0
TABLE_OFFSET_HEIGHT_BEGIN = 4
SITES = [
    {'rva': 0x2F2288, 'expect': 'BDF8E9FD00', 'target': 'begin', 'operand': 1},
    {'rva': 0x2F2290, 'expect': '837C2410067779', 'target': 'nop', 'operand': 0},
    {'rva': 0x2F2317, 'expect': '81FD30EAFD000F8C6DFFFFFF', 'target': 'end', 'operand': 2},
    {'rva': 0x585392, 'expect': 'B8FCE9FD00', 'target': 'height_begin', 'operand': 1},
    {'rva': 0x5853A3, 'expect': '3D34EAFD00', 'target': 'height_end', 'operand': 1},
]
# 原生判定函数 __thiscall(uint8_t)(graphics, width, height)，调用方看 AL，callee 弹 8 字节参数。
PREDICATE_RVA = GUARD_RVA
PREDICATE_FORCE = bytes.fromhex('B001C20800')    # mov al,1 ; ret 8

ENUM_CURRENT_SETTINGS = 0xFFFFFFFF          # EnumDisplaySettings：当前显示模式
MEM_COMMIT_RESERVE = 0x3000
PAGE_READWRITE = 0x04
PAGE_EXECUTE_READWRITE = 0x40
TABLE_ADDRESSES = (0x10000000, 0x14000000, 0x20000000, 0x30000000)
MAXIMUM_WIDTH = 1920                           # 真机实测：超过 1080p 画面失真（cn-717 没有画布缩放）
MAXIMUM_HEIGHT = 1080
MAX_TABLE_MODES = 64


class PatchError(Exception):
    """补丁点与已知的 cn-717 映像不符。"""


def _table_offset(target, table_size):
    return {'begin': TABLE_OFFSET_BEGIN, 'end': table_size,
            'height_begin': TABLE_OFFSET_HEIGHT_BEGIN,
            'height_end': table_size + TABLE_OFFSET_HEIGHT_BEGIN}.get(target)


def rva_offset(image, rva):
    """把 RVA 映射成文件偏移（只用 PE 节表，够自检用）。"""
    if len(image) < 0x40 or image[:2] != b'MZ':
        raise PatchError('not an MZ image')
    pe = struct.unpack_from('<I', image, 0x3C)[0]
    if image[pe:pe + 4] != b'PE\x00\x00':
        raise PatchError('no PE signature')
    sections = struct.unpack_from('<H', image, pe + 6)[0]
    optional = pe + 24
    size_of_headers = struct.unpack_from('<I', image, optional + 60)[0]
    if rva < size_of_headers:
        return rva
    table = optional + struct.unpack_from('<H', image, pe + 20)[0]
    for index in range(sections):
        entry = table + index * 40
        virtual_size, virtual_address = struct.unpack_from('<II', image, entry + 8)
        raw_size, raw_pointer = struct.unpack_from('<II', image, entry + 16)
        span = max(virtual_size, raw_size)
        if virtual_address <= rva < virtual_address + span:
            return raw_pointer + (rva - virtual_address)
    raise PatchError('RVA 0x{:X} is outside every section'.format(rva))


def check_image(image):
    """静态核对：返回 (timestamp, size_of_image)；不符就抛 PatchError。"""
    pe = struct.unpack_from('<I', image, 0x3C)[0]
    machine = struct.unpack_from('<H', image, pe + 4)[0]
    timestamp = struct.unpack_from('<I', image, pe + 8)[0]
    optional = pe + 24
    size_of_image = struct.unpack_from('<I', image, optional + 56)[0]
    image_base = struct.unpack_from('<I', image, optional + 28)[0]
    if machine != 0x014C:
        raise PatchError('expected the x86 image, got machine 0x{:04X}'.format(machine))
    if image_base != IMAGE_BASE:
        raise PatchError('expected image base 0x{:X}, got 0x{:X}'.format(IMAGE_BASE, image_base))
    if timestamp != TIME_DATE_STAMP:
        raise PatchError('TimeDateStamp 0x{:08X} is not the audited cn-717 build 0x{:08X}'
                         .format(timestamp, TIME_DATE_STAMP))
    if size_of_image not in IMAGE_SIZES:
        raise PatchError('SizeOfImage 0x{:X} is not one of {}'.format(
            size_of_image, ', '.join(hex(value) for value in IMAGE_SIZES)))
    for site in SITES:
        offset = rva_offset(image, site['rva'])
        expected = bytes.fromhex(site['expect'])
        if image[offset:offset + len(expected)] != expected:
            raise PatchError('site 0x{:X} does not carry the audited bytes'.format(site['rva']))
    offset = rva_offset(image, GUARD_RVA)
    if image[offset:offset + len(GUARD)] != GUARD:
        raise PatchError('display predicate 0x{:X} does not carry the audited bytes'.format(GUARD_RVA))
    return timestamp, size_of_image


# ----------------------------------------------------------------------------
# 显示模式：枚举真实支持的模式，按插件同一套规则筛选
# ----------------------------------------------------------------------------

class _DevMode(ctypes.Structure):
    # DEVMODEW 无指针，32/64 位布局一致；只关心 dmPelsWidth / dmPelsHeight。
    _fields_ = [('storage', ctypes.c_byte * 220)]


def display_modes():
    try:
        user32 = ctypes.WinDLL('user32', use_last_error=True)
    except (OSError, AttributeError):
        return set()
    user32.EnumDisplaySettingsW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, ctypes.POINTER(_DevMode)]
    user32.EnumDisplaySettingsW.restype = wintypes.BOOL
    modes = set()
    index = 0
    while index < 512:
        # 文档要求调用前把 dmSize 设成 sizeof(DEVMODE)，否则个别系统会直接返回失败。
        seed = bytearray(220)
        struct.pack_into('<H', seed, 68, 220)          # dmSize
        dev = _DevMode.from_buffer_copy(bytes(seed))
        if not user32.EnumDisplaySettingsW(None, index, ctypes.byref(dev)):
            break
        index += 1
        raw = bytes(dev.storage)
        width, height = struct.unpack_from('<II', raw, 172)   # dmPelsWidth / dmPelsHeight
        if width and height:
            modes.add((width, height))
    return modes


def desktop_size():
    """当前显示模式的物理分辨率。

    GetSystemMetrics 对非 DPI 感知的进程返回的是**被缩放虚拟化**的尺寸
    （1080p + 125% 缩放 → 1536×864），会把 1920×1080 错挡在「不超过桌面」这道过滤之外。
    ENUM_CURRENT_SETTINGS 读的是显示模式本身，永远是物理像素，所以优先走它。
    """
    try:
        user32 = ctypes.WinDLL('user32', use_last_error=True)
        user32.EnumDisplaySettingsW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD,
                                                ctypes.POINTER(_DevMode)]
        user32.EnumDisplaySettingsW.restype = wintypes.BOOL
        seed = bytearray(220)
        struct.pack_into('<H', seed, 68, 220)          # dmSize
        dev = _DevMode.from_buffer_copy(bytes(seed))
        if user32.EnumDisplaySettingsW(None, ENUM_CURRENT_SETTINGS, ctypes.byref(dev)):
            width, height = struct.unpack_from('<II', bytes(dev.storage), 172)
            if width and height:
                return width, height
    except (OSError, AttributeError):
        pass
    try:
        user32 = ctypes.WinDLL('user32', use_last_error=True)
        width, height = user32.GetSystemMetrics(0), user32.GetSystemMetrics(1)
    except (OSError, AttributeError):
        return 0, 0
    if width <= 0 or height <= 0:
        return 0, 0
    return width, height


def make_modes(hardware, custom=CUSTOM_MODES):
    """原表 + 自定义 + 硬件模式。

    规则与插件 make_modes 一致（800x600 例外、>=1024x768、宽高比 < 1780/1000）。
    本实现比插件多两层保守增强，都只会少给档位、不会给出选不了或显示会坏的档位：
      1. 丢弃超过当前桌面尺寸的硬件模式（插件直接相信 D3D 枚举）；
      2. 封顶 1920×1080 —— 客户端画布按 1600×900 设计，真机实测更高就拉伸失真。
    """
    modes = set(ORIGINAL_MODES)
    for width, height in custom:
        if width and height:
            modes.add((width, height))
    limit = desktop_size()
    if not limit[0] or not limit[1]:
        limit = (1 << 31, 1 << 31)
    for width, height in hardware:
        if not height or width > limit[0] or height > limit[1]:
            continue
        # 客户端画布按 1600×900 设计，超过 1080p 只会被拉伸失真，所以封顶。
        if width > MAXIMUM_WIDTH or height > MAXIMUM_HEIGHT:
            continue
        if (width, height) == (800, 600) or (width >= MINIMUM_WIDTH and height >= MINIMUM_HEIGHT):
            if width * 1000 < height * MAXIMUM_ASPECT_MILLI:
                modes.add((width, height))
    return sorted(modes)[:MAX_TABLE_MODES]


# ----------------------------------------------------------------------------
# 进程内补丁
# ----------------------------------------------------------------------------

def _read(kernel, process, address, length):
    buffer = ctypes.create_string_buffer(length)
    copied = ctypes.c_size_t()
    if not kernel.ReadProcessMemory(process, ctypes.c_void_p(address), buffer, length, ctypes.byref(copied)):
        raise PatchError('ReadProcessMemory 0x{:X} failed'.format(address))
    if copied.value != length:
        raise PatchError('short read at 0x{:X}'.format(address))
    return buffer.raw


def _write(kernel, process, address, data):
    previous = wintypes.DWORD()
    if not kernel.VirtualProtectEx(process, ctypes.c_void_p(address), len(data),
                                   PAGE_EXECUTE_READWRITE, ctypes.byref(previous)):
        raise PatchError('VirtualProtectEx 0x{:X} failed'.format(address))
    try:
        copied = ctypes.c_size_t()
        if not kernel.WriteProcessMemory(process, ctypes.c_void_p(address), data, len(data),
                                         ctypes.byref(copied)) or copied.value != len(data):
            raise PatchError('WriteProcessMemory 0x{:X} failed'.format(address))
    finally:
        ignored = wintypes.DWORD()
        kernel.VirtualProtectEx(process, ctypes.c_void_p(address), len(data),
                                previous.value, ctypes.byref(ignored))


def image_ready(kernel, process):
    """核对目标进程的 PE 头与全部补丁点，返回 (是否就绪, 原因)。

    客户端加壳时，补丁点的明文只会在解包之后出现，所以要轮询这个函数而不是只查一次。
    """
    try:
        header = _read(kernel, process, IMAGE_BASE, 0x400)
        if header[:2] != b'MZ':
            return False, 'target image is not mapped at its preferred base'
        pe = struct.unpack_from('<I', header, 0x3C)[0]
        if header[pe:pe + 4] != b'PE\x00\x00':
            return False, 'no PE signature in the target image'
        machine = struct.unpack_from('<H', header, pe + 4)[0]
        timestamp = struct.unpack_from('<I', header, pe + 8)[0]
        size_of_image = struct.unpack_from('<I', header, pe + 24 + 56)[0]
        if machine != 0x014C or timestamp != TIME_DATE_STAMP or size_of_image not in IMAGE_SIZES:
            return False, 'target image is not the audited cn-717 build'
        for site in SITES:
            expected = bytes.fromhex(site['expect'])
            if _read(kernel, process, IMAGE_BASE + site['rva'], len(expected)) != expected:
                return False, 'site 0x{:X} has not been unpacked yet'.format(site['rva'])
        original = b''.join(struct.pack('<II', width, height) for width, height in ORIGINAL_MODES)
        if _read(kernel, process, IMAGE_BASE + TABLE_RVA, len(original)) != original:
            return False, 'the native resolution table is not the audited one'
        if _read(kernel, process, IMAGE_BASE + PREDICATE_RVA, len(GUARD)) != GUARD:
            return False, 'display predicate has not been unpacked yet'
        return True, ''
    except PatchError as error:
        return False, str(error)


def _commit(kernel, process, address, data, undo):
    """写入并回读校验，成功则把原字节压进 undo 供失败时回滚。"""
    original = _read(kernel, process, address, len(data))
    # 先登记再写：即使写到一半失败，回滚也能把这块恢复原样。
    undo.append((address, original))
    _write(kernel, process, address, data)
    if _read(kernel, process, address, len(data)) != data:
        raise PatchError('0x{:X} did not take the write'.format(address))


def _rollback(kernel, process, undo):
    """按相反顺序写回原字节；回滚本身失败只忽略，不掩盖原始错误。"""
    for address, original in reversed(undo):
        try:
            _write(kernel, process, address, original)
        except (PatchError, OSError):
            pass


def _verify_process_image(kernel, process):
    """打补丁前的最后一次核对：不就绪就抛。"""
    ready, reason = image_ready(kernel, process)
    if not ready:
        raise PatchError(reason)


def apply(kernel, process, custom=CUSTOM_MODES):
    """按插件规则把高分辨率表注入进程。返回 (是否成功, 说明)。

    与插件一致：任何一处写失败都整体回滚，不留半改状态。
    """
    undo = []
    try:
        kernel.VirtualAllocEx.argtypes = [wintypes.HANDLE, ctypes.c_void_p, ctypes.c_size_t,
                                          wintypes.DWORD, wintypes.DWORD]
        kernel.VirtualAllocEx.restype = ctypes.c_void_p
        _verify_process_image(kernel, process)
        hardware = display_modes()
        if not hardware:
            # EnumDisplaySettings 枚举不出来时，至少把当前桌面分辨率当成一个候选
            # （它一定被显示器支持），否则回退表等于原表，功能等于没开。
            size = desktop_size()
            hardware = {size} if size[0] and size[1] else set()
        modes = make_modes(hardware, custom)
        if len(modes) <= len(ORIGINAL_MODES):
            return False, '显示器没有可追加的模式，维持原生分辨率列表'
        table_size = len(modes) * 8
        table_address = 0
        for candidate in TABLE_ADDRESSES:
            allocated = kernel.VirtualAllocEx(process, ctypes.c_void_p(candidate), table_size + 8,
                                              MEM_COMMIT_RESERVE, PAGE_READWRITE)
            if allocated:
                table_address = int(allocated)
                break
        if not table_address:
            allocated = kernel.VirtualAllocEx(process, None, table_size + 8,
                                              MEM_COMMIT_RESERVE, PAGE_READWRITE)
            if not allocated:
                return False, 'VirtualAllocEx 失败，无法建立分辨率表'
            table_address = int(allocated)
        if (table_address >> 31) != ((table_address + table_size + 4) >> 31):
            return False, '分辨率表跨越 2GB 边界，已放弃'
        payload = b''.join(struct.pack('<II', width, height) for width, height in modes)
        _commit(kernel, process, table_address, payload, undo)
        for site in SITES:
            expect = bytes.fromhex(site['expect'])
            if site['target'] == 'nop':
                replacement = b'\x90' * len(expect)
            else:
                replacement = bytearray(expect)
                value = table_address + _table_offset(site['target'], table_size)
                struct.pack_into('<I', replacement, site['operand'], value)
                replacement = bytes(replacement)
            _commit(kernel, process, IMAGE_BASE + site['rva'], replacement, undo)
        if FORCE_PREDICATE:
            _commit(kernel, process, IMAGE_BASE + PREDICATE_RVA, PREDICATE_FORCE, undo)
        largest = modes[-1]          # 表按 (宽, 高) 升序，末项就是最大档
        return True, '最大分辨率扩展到 {}x{}'.format(largest[0], largest[1])
    except (PatchError, OSError) as error:
        _rollback(kernel, process, undo)
        return False, '高分辨率扩展未启用：{}'.format(error)


def _self_check(path):
    with open(path, 'rb') as stream:
        image = stream.read()
    pe = struct.unpack_from('<I', image, 0x3C)[0]
    timestamp = struct.unpack_from('<I', image, pe + 8)[0]
    size_of_image = struct.unpack_from('<I', image, pe + 24 + 56)[0]
    print('PE  TimeDateStamp=0x{:08X}  SizeOfImage=0x{:X}'.format(timestamp, size_of_image))
    print('    原生表 VA=0x{:X}，判定函数 VA=0x{:X}'.format(
        IMAGE_BASE + TABLE_RVA, IMAGE_BASE + PREDICATE_RVA))
    check_image(image)
    print('    5 处补丁点在文件里就是明文，运行期单次校验即可')
    modes = make_modes(display_modes())
    print('    本机可上架模式 {} 档：{}'.format(
        len(modes), ', '.join('{}x{}'.format(w, h) for w, h in modes)))


if __name__ == '__main__':
    import sys
    if len(sys.argv) != 2:
        raise SystemExit('用法: python hidpi.py <BubbleFighter.exe>')
    try:
        _self_check(sys.argv[1])
    except PatchError as failure:
        raise SystemExit('MISMATCH  {}\n'
                         '          （客户端加壳时这是预期结果：文件里的补丁点还是密文，'
                         '只能靠运行期的 image_ready 轮询确认）'.format(failure))
