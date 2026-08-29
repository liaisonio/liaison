#!/usr/bin/env python3
"""
将 SVG 文件转换为 favicon.ico
使用方法: python3 convert_svg_to_ico.py <input.svg> [output.ico]
"""

import sys
import subprocess
import os
from pathlib import Path

def svg_to_ico(svg_path, ico_path=None):
    """将 SVG 转换为 ICO 文件"""
    svg_path = Path(svg_path)
    if not svg_path.exists():
        print(f"错误: 文件不存在: {svg_path}")
        return False
    
    if ico_path is None:
        ico_path = svg_path.parent / "favicon.ico"
    else:
        ico_path = Path(ico_path)
    
    # 检查是否有 rsvg-convert
    rsvg_convert = subprocess.run(["which", "rsvg-convert"], 
                                   capture_output=True, text=True)
    if rsvg_convert.returncode != 0:
        print("错误: 未找到 rsvg-convert 工具")
        print("请安装 librsvg: brew install librsvg")
        return False
    
    # 检查是否有 Python PIL/Pillow
    try:
        from PIL import Image
    except ImportError:
        print("错误: 未安装 Pillow 库")
        print("请安装: pip3 install Pillow")
        return False
    
    # 创建临时目录
    temp_dir = Path("/tmp/svg_to_ico")
    temp_dir.mkdir(exist_ok=True)
    
    # 生成不同尺寸的 PNG
    sizes = [16, 32, 48]
    png_files = []
    
    for size in sizes:
        png_file = temp_dir / f"icon_{size}.png"
        result = subprocess.run([
            "rsvg-convert",
            "-w", str(size),
            "-h", str(size),
            svg_path,
            "-o", str(png_file)
        ])
        
        if result.returncode != 0:
            print(f"错误: 转换 {size}x{size} PNG 失败")
            return False
        
        png_files.append((size, png_file))
    
    # 创建 ICO 文件
    try:
        images = []
        for size, png_file in png_files:
            img = Image.open(png_file)
            images.append(img)
        
        # 保存为 ICO（包含多个尺寸）
        images[0].save(
            ico_path,
            format='ICO',
            sizes=[(img.width, img.height) for img in images]
        )
        
        print(f"成功: 已创建 {ico_path}")
        print(f"  包含尺寸: {', '.join([f'{img.width}x{img.height}' for img in images])}")
        
        # 清理临时文件
        for _, png_file in png_files:
            png_file.unlink()
        
        return True
        
    except Exception as e:
        print(f"错误: 创建 ICO 文件失败: {e}")
        return False

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("使用方法: python3 convert_svg_to_ico.py <input.svg> [output.ico]")
        sys.exit(1)
    
    svg_file = sys.argv[1]
    ico_file = sys.argv[2] if len(sys.argv) > 2 else None
    
    success = svg_to_ico(svg_file, ico_file)
    sys.exit(0 if success else 1)
