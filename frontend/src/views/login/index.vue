<template>
  <div class="login-page">
    <div class="login-panel">
      <aside class="brand">
        <div class="brand-logo">
          <span class="logo-mark">N</span>
          <span class="logo-name">Nanyi CRM</span>
        </div>
        <h1 class="brand-slogan">客户关系<br />管理系统</h1>
        <p class="brand-desc">线索、客户、跟进、成交，一站式销售协作平台。</p>
        <div class="brand-chart" aria-hidden="true">
          <i style="height: 34%"></i>
          <i style="height: 52%"></i>
          <i style="height: 41%"></i>
          <i style="height: 68%"></i>
          <i style="height: 57%"></i>
          <i style="height: 84%"></i>
          <i style="height: 72%"></i>
        </div>
        <ul class="brand-points">
          <li>客户 360° 视图</li>
          <li>销售流程可视化</li>
          <li>数据驱动决策</li>
        </ul>
      </aside>

      <section class="form-side">
        <h2 class="form-title">欢迎回来</h2>
        <p class="form-subtitle">请使用账号密码登录系统</p>
        <el-form ref="formRef" :model="form" :rules="rules" size="large" @keyup.enter="onSubmit">
          <el-form-item prop="username">
            <el-input v-model="form.username" placeholder="用户名" :prefix-icon="User" />
          </el-form-item>
          <el-form-item prop="pwd">
            <el-input v-model="form.pwd" type="Password" placeholder="密码" show-Password :prefix-icon="Lock" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" class="submit" :loading="loading" @click="onSubmit">登 录</el-button>
          </el-form-item>
        </el-form>
        <p class="form-footer">Nanyi CRM · 内部管理系统</p>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import { ElMessage, type FormInstance, type FormRules } from "element-plus"
import { User, Lock } from "@element-plus/icons-vue"
import { login, fetchProfile } from "@/api/auth"
import { useUserStore } from "@/store/user"

const router = useRouter()
const route = useRoute()
const store = useUserStore()

const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({ username: "", pwd: "" })

const rules: FormRules = {
  username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  pwd: [{ required: true, message: "请输入密码", trigger: "blur" }],
}

async function onSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    const tokens = await login(form.username, form.pwd)
    store.setTokens(tokens)
    store.setProfile(await fetchProfile())
    ElMessage.success("登录成功")
    const redirect = (route.query.redirect as string) || "/"
    router.push(redirect)
  } catch {
    // error toast handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background-color: #eef1f4;
  background-image:
    linear-gradient(rgba(15, 118, 110, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(15, 118, 110, 0.045) 1px, transparent 1px);
  background-size: 32px 32px;
}

.login-panel {
  display: flex;
  width: 860px;
  max-width: 100%;
  min-height: 520px;
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 12px 40px rgba(15, 23, 42, 0.12);
}

/* ---- brand side ---- */
.brand {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 44px 40px;
  color: #ecfdf5;
  background: linear-gradient(160deg, #0f766e 0%, #115e59 55%, #134e4a 100%);
}
.brand-logo {
  display: flex;
  align-items: center;
  gap: 10px;
}
.logo-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.16);
  border: 1px solid rgba(255, 255, 255, 0.3);
  font-size: 18px;
  font-weight: 700;
}
.logo-name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.5px;
}
.brand-slogan {
  margin: 56px 0 0;
  font-size: 30px;
  line-height: 1.4;
  font-weight: 700;
  color: #fff;
}
.brand-desc {
  margin: 14px 0 0;
  font-size: 13px;
  color: rgba(236, 253, 245, 0.75);
}
.brand-chart {
  margin-top: auto;
  display: flex;
  align-items: flex-end;
  gap: 10px;
  height: 88px;
}
.brand-chart i {
  flex: 1;
  border-radius: 4px 4px 0 0;
  background: linear-gradient(to top, rgba(255, 255, 255, 0.35), rgba(255, 255, 255, 0.12));
}
.brand-points {
  margin: 24px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.brand-points li {
  font-size: 12px;
  padding: 5px 12px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: rgba(236, 253, 245, 0.9);
}

/* ---- form side ---- */
.form-side {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 44px 48px;
}
.form-title {
  margin: 0;
  font-size: 24px;
  color: #1f2937;
}
.form-subtitle {
  margin: 6px 0 28px;
  font-size: 13px;
  color: #909399;
}
.submit {
  width: 100%;
  letter-spacing: 6px;
}
.form-footer {
  margin: 32px 0 0;
  text-align: center;
  font-size: 12px;
  color: #c0c4cc;
}

@media (max-width: 720px) {
  .brand {
    display: none;
  }
  .login-panel {
    min-height: auto;
  }
}
</style>

