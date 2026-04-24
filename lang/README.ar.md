<div dir="rtl">

# Codg

<p align="left">
    <a href="https://github.com/vcaesar/codg/releases"><img src="https://img.shields.io/github/release/vcaesar/codg" alt="أحدث إصدار"></a>
    <a href="https://github.com/vcaesar/codg/actions"><img src="https://github.com/vcaesar/codg/actions/workflows/go.yml/badge.svg" alt="حالة البناء"></a>
    <a href="https://pkg.go.dev/github.com/vcaesar/codg?tab=doc"><img src="https://pkg.go.dev/badge/github.com/vcaesar/codg?status.svg" alt="GoDoc"></a>
    <a href="https://discord.gg/Dy5QZRbaND"><img src="https://img.shields.io/discord/1484658282777018551.svg?logo=discord&logoColor=white&label=Discord&color=5865F2" alt="انضم إلى دردشة Discord على https://discord.gg/Dy5QZRbaND"></a>
</p>

نظام Harness وكلاء ذكاء اصطناعي للجيل القادم، سهل الاستخدام للبرمجة والعمل: تلقائي وغير متزامن، بتزامن عالٍ وأداء مرتفع، وبكفاءة ودقة عاليتين.

[English](../README.md) | [繁體中文](./README.zht.md) | [简体中文](./README.zh.md) | [日本語](./README.ja.md) | [한국어](./README.ko.md) | [Français](./README.fr.md) | [Deutsch](./README.de.md) | [Español](./README.es.md) | [Português](./README.pt.md) | [Русский](./README.ru.md) | **العربية**

<p align="center">
<a href="https://atomai.cc" rel="nofollow">
<img width="800" alt="عرض Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04.png" />
<img width="800" alt="عرض Codg" src="https://github.com/vcaesar/codg/raw/main/demo/26-04-1.png" />
</a>
</p>

## التثبيت

ماك ولينكس:

```bash
# Homebrew
brew install vcaesar/tap/codg

# NPM
# npm install -g @vcaesar/codg
```

ويندوز (PowerShell):

```powershell
# Winget
# winget install vcaesar.codg

# YOLO (مثبّت PowerShell الأصلي)
irm https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.ps1 | iex
```

الكل (macOS أو Linux أو Windows عبر Git Bash / MSYS2 / Cygwin / WSL):

```bash
# YOLO
curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash
```

أو انقر مباشرةً على [الإصدارات](https://github.com/vcaesar/codg/releases) لتنزيله وتشغيله.

انتقل إلى مجلد مشروعك ثم شغّل `codg`.
استخدم "/yolo" للتبديل بين الوضع التلقائي ووضع التأكيد، ويمكن ضبط الأذونات من خلال codg.toml.

## الميزات

- نظام وكلاء تلقائي وغير متزامن، عالي التزامن والأداء، مع استهلاك منخفض للذاكرة
- دعم مزودي نماذج متعددين (API و Pro) ونماذج محلية عبر openai-compat أو claude-compat، مع دعم نماذج Openrouter و Ollama و Nvidia وغيرها المجانية، استخدمها عبر "/connect" أو "/models" أو "codg auth"
- دعم أي طرفية ونظام تشغيل، بالإضافة إلى دعم طرفيات الويب
- سهل الاستخدام: واجهة TUI متاحة في كل مكان وقريبة من GUI؛ إصدار سطح المكتب والويب في مرحلة BETA
- انقر أو استخدم "/xxx" لتبديل الجلسات، كل شيء في TUI قابل للنقر
- انقر على "Modified Files" أو استخدم "/diff" و"/diff git" لعرض ملفات الفرق داخل TUI كما في VSCode
- إكمال تلقائي للحروف الإنجليزية والعبارات القصيرة

تطبيق سطح المكتب (BETA)، ويب (BETA)، Claw (BETA)، بعض الميزات لا تزال بحاجة إلى اختبار وإصلاح للأخطاء ثم إصدارها.

## المعيار

### استخدام الذاكرة (RAM)

| الأداة                 | جلسة نشطة واحدة  | 10 جلسات نشطة       | PSS إضافي لكل جلسة مضافة   |
| ---------------------- | ---------------- | ------------------- | -------------------------- |
| **Codg**               | 65 MB            | 165 MB              | ~10 MB                     |
| **Codex CLI**          | 140.0 MB         | 334.8 MB            | ~21.6 MB                   |
| **Cursor Agent**       | 214.9 MB         | 1632.4 MB           | ~157.5 MB                  |
| **GitHub Copilot CLI** | 333.3 MB         | 1756.5 MB           | ~158.1 MB                  |
| **OpenCode**           | 371.5 MB         | 3237.2 MB           | ~318.4 MB                  |
| **Claude Code**        | 386.6 MB         | 2300.6 MB           | ~212.7 MB                  |

## الإبلاغ عن الأخطاء:

افتح [Github Issue](https://github.com/vcaesar/codg/issues)

## كيف نستخدم بياناتك:

حاليًا لا يتم جمع أي بيانات أو قياسات، كما يتم دعم النماذج المحلية بنسبة 100%. عند استخدام واجهة API، يرجى مراجعة سياسات المزوّد المعني.

# أوامر CLI

استخدم `codg -h` أو "/help" في TUI

```bash
codg auth/login               # المصادقة (Atom، OpenAI، GitHub...)
codg web                      # تشغيل واجهة الويب على المنفذ 4096
codg desktop                  # تشغيل تطبيق سطح المكتب (Wails)
codg claw                     # تشغيل وكيل المراسلة (Telegram/Discord/Slack)
codg gateway --private-only   # تشغيل بوابة مؤمّنة
codg models claude            # عرض النماذج المطابقة لـ "claude"
codg runm start Qwen/Qwen3-8B-GGUF   # تشغيل نموذج محلي
codg runm download user/model # تنزيل نموذج GGUF
codg plugin install repo/name # تثبيت مكوّن إضافي
codg plugin list              # عرض المكوّنات المثبّتة
codg install repo/name        # اختصار لـ plugin install
codg mcp add myserver cmd     # إضافة خادم MCP
codg mcp list                 # عرض خوادم MCP المكوّنة
codg skill url add <url>      # إضافة رابط مصدر المهارات
codg themes set catppuccin    # تبديل السمة
# codg logs -f                # متابعة سجلات التطبيق
codg toml                     # عرض الإعدادات بالكامل
codg stats/s                  # عرض إحصائيات الاستخدام
codg dirs                     # طباعة مسارات البيانات/الإعدادات
codg projects                 # عرض مجلدات المشاريع المتتبَّعة
codg lite 2                   # تعيين مستوى الوضع الخفيف (0-4)
codg merge origin main        # دمج git آمن مع نسخة احتياطية v1/
codg migrate                  # ترحيل الإعدادات من .claude/.opencode
codg vm build                 # البناء على جهاز افتراضي بعيد
codg vm run -- make test      # تنفيذ أمر على الـ VM
codg sandbox run -- ./test.sh # التشغيل في بيئة الحماية
codg sandbox status           # التحقق من توفر الـ sandbox
codg update                   # تحديث تعريفات المزوّدين
```

## أمثلة الاستخدام

### الوضع غير التفاعلي (`codg run`)

```bash
# تمرير المدخلات من أمر آخر.
cat errors.log | codg run "ما سبب هذه الأخطاء؟"
# الوضع المفصّل (إخراج التصحيح إلى stderr).
codg run -v "تصحيح هذه الدالة"
```

### واجهة الويب

```bash
# تشغيل واجهة الويب على المنفذ الافتراضي 4096؛ (بعد اكتمال الاختبارات، قم بإصدارها).
codg web
# منفذ مخصص.
codg web -p 8080

# وضع API فقط (بدون واجهة أمامية أو متصفح).
codg web 0
```

### إدارة المكوّنات الإضافية

```bash
# تثبيت مكوّن إضافي من مستودع Git.
codg install github.com/user/codg-xxx-auth
```

### الوكلاء والمهارات المخصصة:

انسخ xx_agent.md (.codg/agents/templates) أو SKILL.md (.codg/skills) إلى المجلد المناسب

# نظام الإعدادات

أنشئ ملف `codg.toml` في جذر المشروع (أو `~/.codg/config/codg.toml` للإعدادات العامة):

```toml
# codg.toml — إعداد الحد الأدنى للمشروع.
[options]
lite_mode = 0          # 0 = جميع الوكلاء، 2 = المجموعة الافتراضية المبسطة، 4 = وكيل واحد
locale    = "en"       # لغة الواجهة: en، zh-CN، ja

[options.tui]
theme     = "catppuccin"
dark_mode = true
compact_mode = false
```

### إعداد المزوّد

```toml
# استخدام مفتاح API (يدعم توسعة $ENV_VAR).
[providers.anthropic]
api_key = "$ANTHROPIC_API_KEY"

# استخدام OAuth (يُضبط عبر `codg auth`).
[providers.openai]
oauth = true

# مزوّد مخصّص / مستضاف ذاتياً.
[providers.local]
name     = "My Local LLM"
type     = "openai-compat"
base_url = "http://localhost:8080/v1"
api_key  = "not-needed"
```

### تخصيص الوكلاء

```toml
# الصيغة المختصرة: تعيين نوع النموذج.
agents.coder = "large"
agents.task  = "small"

# الصيغة الكاملة: ضبط الوكيل بدقة.
[agents.advisor]
model           = "large"
temperature     = 0.3
thinking_budget = 32000
```

### خوادم MCP

```toml
# خادم MCP عبر HTTP.
[mcp.websearch]
type = "http"
url  = "https://mcp.exa.ai/mcp?tools=web_search_exa"
```

### المهارات

```toml
# التحميل والتنزيل التلقائي في TUI أو عبر codg skill
[option]
skill_urls = ["https://github.com/user/skills"]
```

### النماذج المحلية (llama.cpp)

```toml
[llama]
port     = 8090
host     = "127.0.0.1"
ctx_size = 32000
gpu      = "auto"          # auto، cuda، off
```

### قنوات المراسلة

```toml
[channels.telegram]
enabled     = true
token       = "$TELEGRAM_BOT_TOKEN"
allowed_ids = ["123456789"]

[channels.discord]
enabled  = true
token    = "$DISCORD_BOT_TOKEN"
```

### الصلاحيات

```toml
[permissions]
allowed_tools = ["bash", "edit", "view", "glob", "grep"]
allowed_dirs = ["**x"] # جميع المجلدات
```

</div>
