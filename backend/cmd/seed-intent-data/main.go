// seed-intent-data creates deterministic CRM fixtures for AI intent testing.
// It is safe to run repeatedly: customers with the fixture prefix are reused.
// Set SEED_RESET=true to remove only these generated fixtures before seeding.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/database"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
	"gorm.io/gorm"
)

const fixturePrefix = "AI测试客户-"

type followFixture struct {
	kind    int8
	content string
	nextIn  int
}

type opportunityFixture struct {
	name     string
	stage    int8
	amount   float64
	expectIn int
	remark   string
}

type customerFixture struct {
	group       string
	industry    string
	source      string
	level       string
	status      int8
	remark      string
	contact     string
	position    string
	follows     []followFixture
	opportunity *opportunityFixture
}

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := database.NewMySQL(cfg.DefaultMySQL(), false)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	tenantID := envUint64("SEED_TENANT_ID", 1)
	ownerID := envUint64("SEED_OWNER_ID", 1)
	created, reused, children := 0, 0, 0
	now := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60))

	err = db.Transaction(func(tx *gorm.DB) error {
		if os.Getenv("SEED_RESET") == "true" {
			if err := resetFixtures(tx, tenantID); err != nil {
				return err
			}
		}
		for index, fixture := range fixtures() {
			name := fmt.Sprintf("%s%s-%02d", fixturePrefix, fixture.group, index+1)
			var customer model.CrmCustomer
			findErr := tx.Where("tenant_id = ? AND name = ?", tenantID, name).First(&customer).Error
			switch {
			case findErr == nil:
				reused++
				continue
			case !isNotFound(findErr):
				return findErr
			}

			customer = model.CrmCustomer{
				TenantID: tenantID,
				Name:     name,
				Phone:    fmt.Sprintf("1390000%04d", index+1),
				Source:   fixture.source,
				Industry: fixture.industry,
				Level:    fixture.level,
				Status:   fixture.status,
				OwnerID:  &ownerID,
				Address:  fmt.Sprintf("测试地址-%02d", index+1),
				Remark:   fixture.remark,
			}
			if err := tx.Create(&customer).Error; err != nil {
				return err
			}
			created++

			contact := model.CrmContact{
				TenantID:   tenantID,
				CustomerID: customer.ID,
				Name:       fixture.contact,
				Phone:      fmt.Sprintf("1381000%04d", index+1),
				Email:      fmt.Sprintf("ai-fixture-%02d@example.test", index+1),
				Position:   fixture.position,
				IsPrimary:  1,
				Remark:     "AI意向分析测试联系人",
			}
			if err := tx.Create(&contact).Error; err != nil {
				return err
			}
			children++

			for _, follow := range fixture.follows {
				nextAt := now.AddDate(0, 0, follow.nextIn)
				item := model.CrmFollowUp{
					TenantID:   tenantID,
					CustomerID: customer.ID,
					ContactID:  &contact.ID,
					Type:       follow.kind,
					Content:    follow.content,
					NextAt:     &nextAt,
					CreatorID:  ownerID,
					Creator:    "admin",
				}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
				children++
			}

			if fixture.opportunity != nil {
				expectDate := now.AddDate(0, 0, fixture.opportunity.expectIn)
				opportunity := model.CrmOpportunity{
					TenantID:   tenantID,
					CustomerID: customer.ID,
					Name:       fixture.opportunity.name,
					Stage:      fixture.opportunity.stage,
					Amount:     fixture.opportunity.amount,
					ExpectDate: &expectDate,
					OwnerID:    &ownerID,
					Remark:     fixture.opportunity.remark,
				}
				if err := tx.Create(&opportunity).Error; err != nil {
					return err
				}
				children++
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("seed intent fixtures: %v", err)
	}
	log.Printf("seed intent fixtures completed: created=%d reused=%d related_records=%d prefix=%s tenant=%d",
		created, reused, children, fixturePrefix, tenantID)
}

func envUint64(name string, fallback uint64) uint64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		log.Fatalf("%s must be a positive integer", name)
	}
	return parsed
}

func isNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func resetFixtures(tx *gorm.DB, tenantID uint64) error {
	var customerIDs []uint64
	if err := tx.Model(&model.CrmCustomer{}).
		Where("tenant_id = ? AND name LIKE ?", tenantID, fixturePrefix+"%").
		Pluck("id", &customerIDs).Error; err != nil {
		return err
	}
	if len(customerIDs) == 0 {
		return nil
	}

	for _, item := range []interface{}{
		&model.CrmCustomerIntentFeedback{},
		&model.CrmCustomerIntent{},
		&model.CrmCustomerIntentAnalysis{},
		&model.CrmCustomerIntentTaskItem{},
		&model.CrmContract{},
		&model.CrmOpportunity{},
		&model.CrmFollowUp{},
		&model.CrmContact{},
	} {
		if err := tx.Where("tenant_id = ? AND customer_id IN ?", tenantID, customerIDs).Delete(item).Error; err != nil {
			return err
		}
	}
	return tx.Where("tenant_id = ? AND id IN ?", tenantID, customerIDs).Delete(&model.CrmCustomer{}).Error
}

func fixtures() []customerFixture {
	return []customerFixture{
		{
			group: "高意向", industry: "制造业", source: "转介绍", level: "A", status: 1,
			remark:  "已确认采购数量和预算，要求本月内完成报价与合同评审，计划尽快下单。",
			contact: "张总", position: "采购负责人",
			follows: []followFixture{
				{kind: 1, content: "客户确认采购数量，要求今天补充正式报价。", nextIn: 1},
				{kind: 3, content: "已安排本周方案评审会议，客户将邀请财务和技术负责人参加。", nextIn: 3},
			},
			opportunity: &opportunityFixture{"设备采购项目", 4, 680000, 25, "客户已进入商务谈判，等待最终付款条款确认"},
		},
		{
			group: "高意向", industry: "医疗健康", source: "官网咨询", level: "A", status: 1,
			remark:  "客户急需解决现有系统容量不足问题，预算已获批，希望两周内启动项目。",
			contact: "李经理", position: "信息化负责人",
			follows: []followFixture{
				{kind: 2, content: "现场确认部署环境和接口要求，客户认可整体方案。", nextIn: 2},
				{kind: 1, content: "客户催促发送合同初稿，内部采购流程已启动。", nextIn: 1},
			},
			opportunity: &opportunityFixture{"医疗系统升级项目", 3, 420000, 18, "预算明确，正在确认技术方案"},
		},
		{
			group: "高意向", industry: "教育培训", source: "老客转介绍", level: "A", status: 1,
			remark:  "已完成产品演示和试用，客户反馈良好，正在比较两家供应商，预计下月采购。",
			contact: "王老师", position: "校区负责人",
			follows: []followFixture{
				{kind: 3, content: "客户提出并发量和售后响应问题，已给出解决方案。", nextIn: 4},
				{kind: 4, content: "发送成功案例和报价单，等待客户内部审批意见。", nextIn: 6},
			},
			opportunity: &opportunityFixture{"教学平台采购", 3, 198000, 35, "客户正在进行供应商比选"},
		},
		{
			group: "高意向", industry: "物流运输", source: "销售主动开发", level: "A", status: 1,
			remark:  "客户计划建设新的仓储中心，明确需要采购系统和配套设备，项目时间紧。",
			contact: "赵经理", position: "项目经理",
			follows: []followFixture{
				{kind: 2, content: "完成仓库现场勘察，确认一期实施范围。", nextIn: 2},
				{kind: 1, content: "客户要求本周提供分期交付计划和报价。", nextIn: 1},
			},
			opportunity: &opportunityFixture{"仓储中心建设项目", 4, 950000, 45, "商务条款已基本确认"},
		},
		{
			group: "高意向", industry: "连锁零售", source: "展会获客", level: "A", status: 1,
			remark:  "已锁定门店数量和实施范围，客户要求尽快安排合同签署。",
			contact: "陈总", position: "运营总监",
			follows:     []followFixture{{kind: 1, content: "客户确认最终门店清单，等待合同盖章。", nextIn: 2}},
			opportunity: &opportunityFixture{"连锁门店数字化项目", 4, 520000, 12, "客户已确认采购，待签约"},
		},
		{
			group: "高意向", industry: "建筑工程", source: "合作伙伴推荐", level: "A", status: 1,
			remark:  "项目已立项并安排专项预算，客户需要在月底前完成供应商确定。",
			contact: "刘工", position: "技术负责人",
			follows:     []followFixture{{kind: 3, content: "完成技术交流，客户确认关键功能满足要求。", nextIn: 3}},
			opportunity: &opportunityFixture{"工程项目管理系统", 3, 360000, 28, "项目已立项"},
		},
		{
			group: "高意向", industry: "金融服务", source: "官网咨询", level: "A", status: 1,
			remark:  "客户明确表达采购意向，已提供内部预算和上线时间，等待法务审合同。",
			contact: "周经理", position: "采购经理",
			follows:     []followFixture{{kind: 4, content: "法务提出两处合同修改意见，已反馈销售和客户。", nextIn: 2}},
			opportunity: &opportunityFixture{"客户服务平台项目", 4, 760000, 20, "合同法务审核中"},
		},
		{
			group: "高意向", industry: "食品加工", source: "老客户复购", level: "A", status: 2,
			remark:  "客户已完成采购并进入交付阶段，可用于测试已成交客户的意向分析。",
			contact: "黄经理", position: "厂长",
			follows:     []followFixture{{kind: 2, content: "已完成设备交付验收，客户计划追加采购。", nextIn: 30}},
			opportunity: &opportunityFixture{"生产线升级项目", 5, 240000, 60, "一期已赢单"},
		},
		{
			group: "中意向", industry: "批发零售", source: "转介绍", level: "B", status: 1,
			remark:  "客户认可产品方向，正在内部评估需求和预算，尚未确定采购时间。",
			contact: "郭经理", position: "运营经理",
			follows: []followFixture{
				{kind: 1, content: "客户反馈正在走内部评审流程，暂无明确时间。", nextIn: 10},
				{kind: 4, content: "发送产品案例和功能清单，等待客户反馈。", nextIn: 7},
			},
			opportunity: &opportunityFixture{"零售管理系统", 2, 160000, 75, "需求确认阶段"},
		},
		{
			group: "中意向", industry: "互联网", source: "线上广告", level: "B", status: 1,
			remark:  "客户有明确业务痛点，愿意参加产品演示，但预算和决策人尚未确认。",
			contact: "林经理", position: "产品负责人",
			follows:     []followFixture{{kind: 3, content: "客户参加线上演示，提出数据报表和权限问题。", nextIn: 8}},
			opportunity: &opportunityFixture{"数据分析平台", 2, 220000, 90, "等待客户明确预算"},
		},
		{
			group: "中意向", industry: "物业管理", source: "销售主动开发", level: "B", status: 1,
			remark:  "客户正在收集供应商方案，需求较清晰，但采购计划可能延期。",
			contact: "何主管", position: "信息中心主管",
			follows:     []followFixture{{kind: 2, content: "客户介绍现有系统情况，要求补充迁移方案。", nextIn: 14}},
			opportunity: &opportunityFixture{"物业管理平台", 2, 180000, 120, "供应商方案收集阶段"},
		},
		{
			group: "中意向", industry: "汽车服务", source: "展会获客", level: "B", status: 1,
			remark:  "客户对方案有兴趣，准备组织部门试用，预计下季度决定是否采购。",
			contact: "宋经理", position: "市场负责人",
			follows:     []followFixture{{kind: 4, content: "已开通试用账号并发送操作手册。", nextIn: 12}},
			opportunity: &opportunityFixture{"门店运营工具", 1, 90000, 150, "试用评估阶段"},
		},
		{
			group: "中意向", industry: "医药流通", source: "合作伙伴推荐", level: "B", status: 1,
			remark:  "客户表示今年有采购计划，当前优先进行需求梳理和成本测算。",
			contact: "钱经理", position: "供应链负责人",
			follows:     []followFixture{{kind: 1, content: "客户提供了现有流程资料，正在梳理需求。", nextIn: 15}},
			opportunity: &opportunityFixture{"供应链协同项目", 2, 310000, 100, "成本测算中"},
		},
		{
			group: "中意向", industry: "家居建材", source: "官网咨询", level: "B", status: 1,
			remark:  "客户主动咨询多个功能模块，愿意进一步沟通，但内部决策链较长。",
			contact: "吴经理", position: "行政负责人",
			follows:     []followFixture{{kind: 3, content: "完成初次需求沟通，客户要求补充同行案例。", nextIn: 9}},
			opportunity: &opportunityFixture{"客户管理系统", 1, 125000, 130, "需求初步确认"},
		},
		{
			group: "中意向", industry: "酒店餐饮", source: "老客复购", level: "B", status: 1,
			remark:  "客户有扩店计划，计划评估现有系统是否需要升级，时间尚未确定。",
			contact: "郑总", position: "连锁发展负责人",
			follows:     []followFixture{{kind: 2, content: "沟通扩店计划，客户预计下月确定升级范围。", nextIn: 20}},
			opportunity: &opportunityFixture{"连锁餐饮升级", 1, 280000, 180, "扩店计划评估中"},
		},
		{
			group: "中意向", industry: "机械制造", source: "合作伙伴推荐", level: "B", status: 1,
			remark:  "客户认可数字化改造方向，正在评估不同车间的实施优先级和预算。",
			contact: "孙经理", position: "生产负责人",
			follows:     []followFixture{{kind: 3, content: "完成生产流程调研，客户要求补充分阶段实施方案。", nextIn: 18}},
			opportunity: &opportunityFixture{"生产数字化改造", 2, 260000, 110, "等待确认一期实施范围"},
		},
		{
			group: "低意向", industry: "服装零售", source: "线上广告", level: "C", status: 1,
			remark:  "客户仅了解产品价格，当前没有明确采购计划，后续有需求再联系。",
			contact: "冯女士", position: "店长",
			follows: []followFixture{{kind: 1, content: "客户表示目前暂无采购计划，先保留资料。", nextIn: 45}},
		},
		{
			group: "低意向", industry: "广告传媒", source: "官网咨询", level: "C", status: 3,
			remark:  "客户预算暂时冻结，项目已暂停，短期内没有重启安排。",
			contact: "马先生", position: "业务负责人",
			follows: []followFixture{{kind: 4, content: "客户确认项目暂停，后续预算恢复再联系。", nextIn: 90}},
		},
		{
			group: "低意向", industry: "农林牧渔", source: "销售主动开发", level: "C", status: 1,
			remark:  "客户需求不明确，只是了解行业解决方案，目前没有预算。",
			contact: "何先生", position: "负责人",
			follows: []followFixture{{kind: 1, content: "电话沟通后客户暂不考虑采购。", nextIn: 60}},
		},
		{
			group: "低意向", industry: "房地产", source: "展会获客", level: "C", status: 1,
			remark:  "客户表示需要等集团统一规划，单个项目暂不单独采购。",
			contact: "蒋经理", position: "项目主管",
			follows: []followFixture{{kind: 3, content: "客户建议半年后再了解集团规划进展。", nextIn: 180}},
		},
		{
			group: "低意向", industry: "软件服务", source: "线上广告", level: "C", status: 1,
			remark:  "客户下载资料后未回复，无法确认真实需求和采购预算。",
			contact: "韩女士", position: "运营",
			follows: []followFixture{{kind: 4, content: "发送两次资料，客户暂未反馈。", nextIn: 30}},
		},
		{
			group: "低意向", industry: "服饰加工", source: "合作伙伴推荐", level: "C", status: 1,
			remark:  "客户目前使用其他供应商，暂无替换计划，仅做竞品信息收集。",
			contact: "沈经理", position: "采购",
			follows: []followFixture{{kind: 1, content: "客户暂无替换计划，保持低频维护。", nextIn: 120}},
		},
		{
			group: "低意向", industry: "旅游服务", source: "官网咨询", level: "C", status: 1,
			remark:  "客户只关注基础功能和价格，暂未安排演示，也没有明确时间表。",
			contact: "罗经理", position: "客服主管",
			follows: []followFixture{{kind: 1, content: "客户表示有需要会主动联系。", nextIn: 75}},
		},
		{
			group: "低意向", industry: "批发零售", source: "销售主动开发", level: "C", status: 1,
			remark:  "首次联系未能接通，客户信息较少，暂时无法判断采购意向。",
			contact: "许先生", position: "负责人",
			follows: []followFixture{{kind: 1, content: "首次电话未接通，计划下周再次尝试。", nextIn: 7}},
		},
		{
			group: "未知意向", industry: "", source: "其他", level: "", status: 1,
			remark:  "仅录入了客户名称，暂无有效沟通记录。",
			contact: "待确认", position: "",
		},
		{
			group: "未知意向", industry: "制造业", source: "其他", level: "", status: 1,
			remark:  "客户资料不完整，尚未确认具体需求。",
			contact: "待确认", position: "",
		},
		{
			group: "未知意向", industry: "教育培训", source: "其他", level: "", status: 1,
			remark:  "客户刚导入系统，暂未进行首次跟进。",
			contact: "待确认", position: "",
		},
		{
			group: "未知意向", industry: "建筑工程", source: "其他", level: "", status: 1,
			remark:  "只有基础联系方式，没有预算、时间和决策角色信息。",
			contact: "待确认", position: "",
		},
		{
			group: "未知意向", industry: "物流运输", source: "其他", level: "", status: 1,
			remark:  "客户来源记录不完整，等待销售补充沟通情况。",
			contact: "待确认", position: "",
		},
		{
			group: "未知意向", industry: "", source: "其他", level: "", status: 1,
			remark:  "历史客户数据迁移记录，暂无可用于判断意向的业务信息。",
			contact: "待确认", position: "",
		},
	}
}
