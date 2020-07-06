#!/usr/bin/env python
# 生成用于数字混淆的字体文件用于反爬
# 即字体对于数字的渲染是错误的，例如数字 1 会渲染成 5
# ./genfont.py <font-file> <count>
# 生成字体在 result/generated 目录中

import sys
import os
import subprocess
from pathlib import Path
import random
from bs4 import BeautifulSoup
import copy
import hashlib

names = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"]

# must contain glyphs with name "zero" "one" .. "nine"
def check_font(ttx):
  for name in names:
    if ttx.find("TTGlyph", attrs={"name": name}) is None:
      return False

  return True

def gen(ttx):
  mapping = names[:]
  random.shuffle(mapping)

  target = copy.copy(ttx)

  for name in names:
    target.find("TTGlyph", {"name": name})["id"] = name

  for idx, name in enumerate(names):
    tmp = target.find("TTGlyph", attrs={"id": mapping[idx]})

    tmp.attrs = {}

    for k, v in ttx.find("TTGlyph", attrs={"name": name}).attrs.items():
      tmp[k] = v

  content = target.prettify()
  name = hashlib.md5(content.encode("utf8")).hexdigest()[:10] + "." + "".join([str(names.index(x)) for x in mapping])

  print(f"Generate temporary ttx: {name}.ttx")
  target_ttx_path = os.path.join("result", "tmp", f"{name}.ttx")
  with open(target_ttx_path, "w") as f:
    f.write(content)

  target_ttf_path = os.path.join("result", "generated", f"{name}.ttf")
  print(f"Generate target ttf: {target_ttf_path}")
  subprocess.run(f"ttx -o {target_ttf_path} {target_ttx_path}", shell=True, check=True)

def run(font_file, count):
  ttx_name = os.path.splitext(font_file)[0] + ".ttx"
  ttx_path = os.path.join("result", "tmp", ttx_name)

  if not Path(ttx_path).exists():
    print("Convert ttf to ttx..")
    subprocess.run(f"ttx -o {ttx_path} {font_file}", shell=True, check=True)

  with open(ttx_path) as f:
    ttx = BeautifulSoup(f, "xml")

    if not check_font(ttx):
      print("font must contain glyphs with name 'zero', 'one', 'two' .. 'nine'")
      exit(1)

    for _ in range(count):
      gen(ttx)

if __name__ == "__main__":
  if len(sys.argv) < 3:
    print(f"usage: ./genfont.py <font-file> <count>")
    exit(1)

  # create necessary dirs
  os.makedirs(os.path.join("result", "generated"), exist_ok=True)
  os.makedirs(os.path.join("result", "tmp"), exist_ok=True)

  run(sys.argv[1], int(sys.argv[2]))
