FROM ubuntu:20.04

# 更新包列表并安装必要的软件包
RUN apt-get update \
    && apt-get install -y \
        tzdata \
        fonts-noto-cjk \
        wget

# 安装 Chromium 作为 deb 包而不是 Snap 包
RUN wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb \
    && apt-get install -y ./google-chrome-stable_current_amd64.deb \
    && rm google-chrome-stable_current_amd64.deb

# 创建 chromium 命令的符号链接指向 google-chrome
RUN ln -s /usr/bin/google-chrome /usr/bin/chromium

# 设置时区和环境变量
ENV TZ=Asia/Shanghai

# 验证时区设置是否成功
RUN ls /usr/share/zoneinfo/Asia/Shanghai

# 将应用程序和测试文件复制到镜像中
COPY bin/inspection-server /inspection-server
COPY package/test.pdf /opt/test.pdf
COPY agent/yaml /opt/yaml

# 指定容器启动时运行的命令
CMD ["/inspection-server"]
