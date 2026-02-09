import { UserManageCard } from "../components/UserManageCard";
import { StatusCard } from "../components/StatusCard";
import { LinkManageCard } from "../components/LinkManageCard";
import { useGetConfigs } from "../apis/config";
import { Button } from "../components/ui/button";
import { Modal } from "../components/Modal";
import { SettingsCard } from "../components/SettingsCard";
import { LetsEncrypt } from "./start/letsencrypt";
export default function Index() {
    const { data: config, loaded } = useGetConfigs()
    return (
        <div className='grid grid-cols-1 gap-4'>
            {loaded && !config?.ssl && (
                <div className='bg-yellow-500/10 rounded-lg p-4 border border-yellow-500/20'>
                    <div className='flex items-center justify-between'>
                        <div>
                            <div className='text-lg'>SSL 证书未配置</div>
                            <div className='text-sm opacity-70'>请先配置 SSL 证书以加密你的连接，如未配置您的连接信息可能会被政府或中间人窃取。</div>
                        </div>
                        <Modal
                            content={(
                                <LetsEncrypt />
                            )}
                            title={'配置 SSL'}
                        >
                            <Button variant='outline' className='cursor-pointer rounded-full bg-yellow-500 hover:bg-yellow-500/80'>配置 SSL</Button>
                        </Modal>
                    </div>
                </div>
            )}
            {loaded && !config?.has_password && (
                <div className='bg-yellow-500/10 rounded-lg p-4 border border-yellow-500/20 flex items-center justify-between'>
                    <div>
                        <div className='text-lg'>密码未设置</div>
                        <div className='text-sm opacity-70'>请设置密码以保护你的数据。</div>
                    </div>
                    <Modal
                        content={(
                            <SettingsCard />
                        )}
                        title={'系统设置'}
                    >
                        <Button variant='outline' className='cursor-pointer rounded-full bg-yellow-500 hover:bg-yellow-500/80'>设置密码</Button>
                    </Modal>
                </div>
            )}
            <StatusCard />
            <UserManageCard />
            <LinkManageCard />
        </div>
    )
}