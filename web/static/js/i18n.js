const I18N_MESSAGES = {
  "zh-CN": {
    nav_home: "首页",
    nav_dashboard: "仪表盘",
    nav_settings: "全局设置",
    nav_streams_list: "流列表",
    nav_add_stream: "添加流",
    nav_full_multiview: "全屏多画面",
    nav_docs: "文档",
    nav_misc: "其他",
    nav_navigation: "导航",
    add_stream_btn: "添加流",
    logout: "退出登录",
    settings_title: "全局设置",
    settings_general: "通用设置",
    settings_recording: "录像设置",
    settings_share: "分享默认设置",
    settings_security: "安全设置",
    settings_lang: "界面语言",
    settings_login: "管理员账号",
    settings_session_ttl: "会话有效期(分钟)",
    settings_record_path: "录像保存路径",
    settings_record_format: "格式",
    settings_record_rule: "命名规则",
    settings_record_max: "单次最大录像时长(分钟)",
    settings_resolution: "分辨率(预留)",
    settings_codec: "视频编码(预留)",
    settings_share_expire: "默认有效时间(分钟)",
    settings_share_conn: "默认最大连接数(1-5)",
    settings_old_password: "旧密码",
    settings_new_password: "新密码",
    settings_update_password: "修改管理员密码",
    share_video: "分享视频",
    share_modal_title: "分享视频",
    share_minutes: "有效时间(分钟)",
    share_password: "分享密码",
    share_max_conn: "允许查看连接数",
    share_link: "分享链接",
    share_qr: "二维码",
    share_create: "生成分享链接",
    share_expired: "分享已过期",
    share_expire_at: "截止时间",
    share_failed: "创建分享失败",
    share_conn_limit: "连接数已满，请稍后查看",
    share_watch_title: "分享视频",
    login_title: "管理登录",
    login_desc: "登录后可访问 RTSPtoWEB 管理后台",
    login_user: "用户名",
    login_pass: "密码",
    login_btn: "登录",
    login_fail: "登录失败，请检查用户名或密码",
    login_fail_title: "登录失败",
    save: "保存",
    saved: "保存成功",
    save_failed: "保存失败",
    error: "错误",
    ok: "确定",
    cancel: "取消",
    loading: "加载中",
    playing: "播放中",
    hls_not_supported: "当前浏览器不支持 HLS 播放",
    rec: "REC",
    stop: "STOP",
    recording_started: "录像已开始",
    recording_stopped: "录像已停止",
    recording_error: "录像操作失败"
  },
  "en-US": {
    nav_home: "Home",
    nav_dashboard: "Dashboard",
    nav_settings: "Global Settings",
    nav_streams_list: "Streams List",
    nav_add_stream: "Add Stream",
    nav_full_multiview: "Full Multiview",
    nav_docs: "Documentation",
    nav_misc: "Miscellaneous",
    nav_navigation: "Navigation",
    add_stream_btn: "Add Stream",
    logout: "Logout",
    settings_title: "Global Settings",
    settings_general: "General",
    settings_recording: "Recording",
    settings_share: "Share Defaults",
    settings_security: "Security",
    settings_lang: "UI Language",
    settings_login: "Admin Username",
    settings_session_ttl: "Session TTL (minutes)",
    settings_record_path: "Recording Save Path",
    settings_record_format: "Format",
    settings_record_rule: "Naming Rule",
    settings_record_max: "Max Recording Duration (minutes)",
    settings_resolution: "Resolution (reserved)",
    settings_codec: "Video Codec (reserved)",
    settings_share_expire: "Default Expire Minutes",
    settings_share_conn: "Default Max Connections (1-5)",
    settings_old_password: "Old Password",
    settings_new_password: "New Password",
    settings_update_password: "Update Admin Password",
    share_video: "Share Video",
    share_modal_title: "Share Video",
    share_minutes: "Valid Time (minutes)",
    share_password: "Share Password",
    share_max_conn: "Max View Connections",
    share_link: "Share Link",
    share_qr: "QR Code",
    share_create: "Generate Share Link",
    share_expired: "Share expired",
    share_expire_at: "Expires at",
    share_failed: "Failed to create share",
    share_conn_limit: "Connection limit reached, try later",
    share_watch_title: "Shared Stream",
    login_title: "Admin Login",
    login_desc: "Login to access RTSPtoWEB console",
    login_user: "Username",
    login_pass: "Password",
    login_btn: "Login",
    login_fail: "Login failed, check username or password",
    login_fail_title: "Login Failed",
    save: "Save",
    saved: "Saved",
    save_failed: "Save failed",
    error: "Error",
    ok: "OK",
    cancel: "Cancel",
    loading: "Loading",
    playing: "Playing",
    hls_not_supported: "Your browser does not support HLS playback",
    rec: "REC",
    stop: "STOP",
    recording_started: "Recording started",
    recording_stopped: "Recording stopped",
    recording_error: "Recording operation failed"
  }
};

function getUILang() {
  const local = window.localStorage.getItem("ui_lang");
  if (local && I18N_MESSAGES[local]) {
    return local;
  }
  if (window.serverUiLang && I18N_MESSAGES[window.serverUiLang]) {
    return window.serverUiLang;
  }
  return "zh-CN";
}

function setUILang(lang) {
  if (!I18N_MESSAGES[lang]) {
    return;
  }
  window.localStorage.setItem("ui_lang", lang);
}

function t(key) {
  const lang = getUILang();
  const table = I18N_MESSAGES[lang] || I18N_MESSAGES["zh-CN"];
  return table[key] || key;
}

function applyI18n(root) {
  const target = root || document;
  target.querySelectorAll("[data-i18n]").forEach(function (el) {
    const key = el.getAttribute("data-i18n");
    el.textContent = t(key);
  });
}

document.addEventListener("DOMContentLoaded", function () {
  applyI18n();
});
