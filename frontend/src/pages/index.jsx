import { UserManageCard } from "../components/UserManageCard";
import { StatusCard } from "../components/StatusCard";
import { LinkManageCard } from "../components/LinkManageCard";
import { useGetConfigs } from "../apis/config";
import { Button } from "../components/ui/button";
import { Modal } from "../components/Modal";
import { SettingsCard } from "../components/SettingsCard";
import { LetsEncrypt } from "./start/letsencrypt";
import { useLanguageStore } from "../store/useLanguageStore";

export default function Index() {
    const { data: config, loaded } = useGetConfigs()
    const { t } = useLanguageStore()
    return (
        <div className='grid grid-cols-1 gap-4'>
            {loaded && !config?.ssl && (
                <div className='bg-yellow-500/10 rounded-lg p-4 border border-yellow-500/20'>
                    <div className='flex items-center justify-between'>
                        <div>
                            <div className='text-lg'>{t('ssl_not_configured')}</div>
                            <div className='text-sm opacity-70'>{t('ssl_warning_index')}</div>
                        </div>
                        <Modal
                            content={(
                                <LetsEncrypt />
                            )}
                            title={t('configure_ssl')}
                        >
                            <Button variant='outline' className='cursor-pointer rounded-full bg-yellow-500 hover:bg-yellow-500/80'>{t('configure_ssl')}</Button>
                        </Modal>
                    </div>
                </div>
            )}
            {loaded && !config?.has_password && (
                <div className='bg-yellow-500/10 rounded-lg p-4 border border-yellow-500/20 flex items-center justify-between'>
                    <div>
                        <div className='text-lg'>{t('password_not_set')}</div>
                        <div className='text-sm opacity-70'>{t('password_warning_desc')}</div>
                    </div>
                    <Modal
                        content={(
                            <SettingsCard />
                        )}
                        title={t('system_settings')}
                    >
                        <Button variant='outline' className='cursor-pointer rounded-full bg-yellow-500 hover:bg-yellow-500/80'>{t('set_password')}</Button>
                    </Modal>
                </div>
            )}
            <StatusCard />
            <UserManageCard />
            <LinkManageCard />
        </div>
    )
}